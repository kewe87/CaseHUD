// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Text.RegularExpressions;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.Cooling
{
    /// <summary>The fan headers of the mainboard that can be set.</summary>
    internal interface IControlPorts
    {
        /// <summary>False: nothing was written, the header is as it was.</summary>
        bool Set(string key, int percent);

        /// <summary>False: the board did not get the header back and it stays set.</summary>
        bool Release(string key);
    }

    /// <summary>What the hardware library needs to know about setting a header.</summary>
    internal static class ControlValues
    {
        public const int ReadBackTolerancePercent = 2;
        public const long ReadBackSettleMs = 5000;

        /// <summary>
        /// The value a header's control starts from. Taking a header over writes the
        /// current value first and the wanted one after it, so full speed is the only
        /// start that cannot leave a fan or pump standing.
        /// </summary>
        public static string Start(string settingName, string fallback) =>
            settingName.StartsWith("/lpc/", StringComparison.Ordinal) && settingName.EndsWith("/control/value", StringComparison.Ordinal)
                ? "100"
                : fallback;

        /// <summary>
        /// Repeating a duty never reaches the chip by itself, so a board that took the
        /// header back (after standby, say) only shows in what the header really runs at.
        /// </summary>
        public static bool MustRewrite(int wanted, float? read, long msSinceWrite) =>
            msSinceWrite >= ReadBackSettleMs && read.HasValue && Math.Abs(read.Value - wanted) > ReadBackTolerancePercent;
    }

    /// <summary>
    /// Serves one client of the cooling pipe (\\.\pipe\CaseHUD.Cooling). The only request
    /// accepted is hold: one fan header of the mainboard that an administrator released, at a duty that keeps it turning.
    /// It is repeated while the client lives. A client that ends or goes silent loses its
    /// headers, so a crash of the client leaves the part with the board and not at a frozen duty.
    /// A crash of the service itself is covered by the journal, see ControlHolds.
    /// </summary>
    internal sealed class CoolingConnection
    {
        public const string PipeName = "CaseHUD.Cooling";
        public const int LowestPercent = 30;
        public const int MaxBody = 256;
        public const double RequestsPerSecond = 10;
        public const long SilentMs = 15000;
        public const int SweepMs = 2000;

        internal static readonly Regex KeyPattern = new Regex(@"\A/lpc/[a-z0-9]{1,32}/[0-9]{1,2}/control/[0-9]{1,2}\z", RegexOptions.CultureInvariant);
        private static readonly HashSet<string> Fields = new HashSet<string> { "v", "type", "key", "percent" };

        private readonly ControlHolds _holds;
        private readonly DenyLog _denyLog;
        private readonly Func<string, bool> _allowed;
        private readonly RateLimiter _limiter;
        private readonly RateLimiter _shared;

        public CoolingConnection(ControlHolds holds, DenyLog denyLog, Func<string, bool> allowed, RateLimiter limiter = null, RateLimiter shared = null)
        {
            _holds = holds;
            _denyLog = denyLog;
            _allowed = allowed;
            _limiter = limiter ?? new RateLimiter(RequestsPerSecond);
            _shared = shared;
        }

        /// <summary>
        /// The always-on cooling pipe: same server class and ACL as the other pipes, shorter idle limit.
        /// clientAllowed decides by the client's process, which the ACL does not: it admits every interactive user.
        /// </summary>
        public static SmbusBridgeHost CreateHost(ControlHolds holds, Func<string, bool> allowed, Action<string> log, Func<int, bool> clientAllowed)
        {
            // The host and the server were written for the SMBus bridge and say so in their log lines.
            Action<string> own = line => log(line.Replace("SMBus bridge", "cooling pipe").Replace(SmbusBridgeHost.PipeName, PipeName));
            var denyLog = new DenyLog(own);
            var shared = new RateLimiter(RequestsPerSecond * 2);
            return new SmbusBridgeHost(
                () => true,
                () => new NamedPipeBridgeServer(PipeName, s => new CoolingConnection(holds, denyLog, allowed, null, shared).Serve(s), own, idleMs: SilentMs, sweepMs: SweepMs, clientAllowed: clientAllowed),
                own);
        }

        public void Serve(Stream s)
        {
            try
            {
                while (true)
                {
                    byte[] body;
                    BridgeProtocol.ReadResult r = BridgeProtocol.ReadMessage(s, out body);
                    if (r == BridgeProtocol.ReadResult.Invalid) _denyLog.Denied("cooling pipe: message size not allowed");
                    if (r != BridgeProtocol.ReadResult.Ok) return;
                    BridgeProtocol.WriteMessage(s, Handle(body));
                }
            }
            finally
            {
                _holds.Release(this);
            }
        }

        private byte[] Handle(byte[] body)
        {
            if (!_limiter.Allow()) return BridgeProtocol.Error("timeout");
            string reason = null, key = null;
            int percent = 0;
            if (body.Length > MaxBody) reason = "request too large";
            else
            {
                Dictionary<string, object> root = BridgeProtocol.ParseRoot(body, out reason);
                if (root != null) reason = Parse(root, out key, out percent);
            }
            if (reason != null)
            {
                _denyLog.Denied("cooling pipe: " + reason);
                return BridgeProtocol.Error("denied");
            }
            if (!_allowed(key))
            {
                _denyLog.Denied("cooling pipe: " + key + " is not released");
                return BridgeProtocol.Error("denied");
            }
            if (_shared != null && !_shared.Allow()) return BridgeProtocol.Error("timeout");
            return _holds.Hold(this, key, percent) ? BridgeProtocol.Ok(new byte[0]) : BridgeProtocol.Error("io");
        }

        private static string Parse(Dictionary<string, object> root, out string key, out int percent)
        {
            key = null;
            percent = 0;
            foreach (string name in root.Keys)
            {
                if (!Fields.Contains(name)) return "unknown field";
            }
            object o;
            if (!root.TryGetValue("type", out o) || !(o is string) || (string)o != "hold") return "request type not allowed";
            if (!root.TryGetValue("v", out o) || !(o is int) || (int)o != 1) return "unsupported version";
            if (!root.TryGetValue("key", out o) || !(o is string) || !KeyPattern.IsMatch((string)o)) return "not a fan header of the mainboard";
            key = (string)o;
            if (!root.TryGetValue("percent", out o) || !(o is int) || (int)o < LowestPercent || (int)o > 100) return "percent out of range";
            percent = (int)o;
            return null;
        }
    }
}
