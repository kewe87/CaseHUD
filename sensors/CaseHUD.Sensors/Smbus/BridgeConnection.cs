// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System.IO;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>Serves one client: request in, response out, until it disconnects or breaks the framing.</summary>
    internal sealed class BridgeConnection
    {
        public const double BatchesPerSecond = 120;

        private readonly BridgeExecutor _executor;
        private readonly DenyLog _denyLog;
        public const double TotalBatchesPerSecond = 240;

        private readonly RateLimiter _limiter;
        private readonly RateLimiter _shared;

        /// <param name="shared">Limiter shared by all connections of the service (optional).</param>
        public BridgeConnection(BridgeExecutor executor, DenyLog denyLog, RateLimiter limiter = null, RateLimiter shared = null)
        {
            _executor = executor;
            _denyLog = denyLog;
            _limiter = limiter ?? new RateLimiter(BatchesPerSecond);
            _shared = shared;
        }

        public void Serve(Stream s)
        {
            while (true)
            {
                byte[] body;
                BridgeProtocol.ReadResult r = BridgeProtocol.ReadMessage(s, out body);
                if (r == BridgeProtocol.ReadResult.Invalid) _denyLog.Denied("message size not allowed");
                if (r != BridgeProtocol.ReadResult.Ok) return;

                byte[] resp;
                if (!_limiter.Allow() || (_shared != null && !_shared.Allow()))
                {
                    resp = BridgeProtocol.Error("timeout");
                }
                else
                {
                    string reason;
                    BridgeRequest req = BridgeProtocol.Parse(body, out reason);
                    if (req == null)
                    {
                        _denyLog.Denied(reason);
                        resp = BridgeProtocol.Error("denied");
                    }
                    else
                    {
                        resp = _executor.Execute(req);
                    }
                }
                BridgeProtocol.WriteMessage(s, resp);
            }
        }
    }
}
