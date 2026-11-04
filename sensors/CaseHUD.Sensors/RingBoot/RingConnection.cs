// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>
    /// Serves one client of the ring pipe (\\.\pipe\CaseHUD.Ring). The only request
    /// accepted is setRingBoot; everything else, SMBus requests included, is denied.
    /// </summary>
    internal sealed class RingConnection
    {
        public const string PipeName = "CaseHUD.Ring";
        public const double RequestsPerSecond = 10;

        private readonly Func<RingBootState, bool> _apply;
        private readonly DenyLog _denyLog;
        private readonly RateLimiter _limiter;
        private readonly RateLimiter _shared;

        public RingConnection(Func<RingBootState, bool> apply, DenyLog denyLog, RateLimiter limiter = null, RateLimiter shared = null)
        {
            _apply = apply;
            _denyLog = denyLog;
            _limiter = limiter ?? new RateLimiter(RequestsPerSecond);
            _shared = shared;
        }

        /// <summary>The always-on ring pipe: same server class, ACL and limits as the SMBus pipe.</summary>
        public static SmbusBridgeHost CreateHost(RingBootRequestHandler handler, Action<string> log)
        {
            var denyLog = new DenyLog(log);
            var shared = new RateLimiter(RequestsPerSecond * 2);
            return new SmbusBridgeHost(
                () => true,
                () => new NamedPipeBridgeServer(PipeName, s => new RingConnection(handler.Handle, denyLog, null, shared).Serve(s), log),
                log);
        }

        public void Serve(Stream s)
        {
            while (true)
            {
                byte[] body;
                BridgeProtocol.ReadResult r = BridgeProtocol.ReadMessage(s, out body);
                if (r == BridgeProtocol.ReadResult.Invalid) _denyLog.Denied("ring pipe: message size not allowed");
                if (r != BridgeProtocol.ReadResult.Ok) return;
                BridgeProtocol.WriteMessage(s, Handle(body));
            }
        }

        private byte[] Handle(byte[] body)
        {
            // Per-connection limit first; the shared limit is only charged for valid requests.
            if (!_limiter.Allow()) return BridgeProtocol.Error("timeout");
            string reason = null;
            RingBootState state = null;
            if (body.Length > RingBootProtocol.MaxBody) reason = "request too large";
            else
            {
                Dictionary<string, object> root = BridgeProtocol.ParseRoot(body, out reason);
                if (root != null) state = RingBootProtocol.Parse(root, out reason);
            }
            if (state == null)
            {
                _denyLog.Denied("ring pipe: " + reason);
                return BridgeProtocol.Error("denied");
            }
            if (_shared != null && !_shared.Allow()) return BridgeProtocol.Error("timeout");
            return _apply(state) ? BridgeProtocol.Ok(new byte[0]) : BridgeProtocol.Error("io");
        }
    }
}
