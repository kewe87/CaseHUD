// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>
    /// The pipe request that sets the ring boot state:
    /// {"v":1,"type":"setRingBoot","mode":"off"|"color"|"none","r":0-255,"g":0-255,"b":0-255}
    /// Nothing else is accepted: no other fields, no paths, no other types.
    /// </summary>
    internal static class RingBootProtocol
    {
        public const string Type = "setRingBoot";
        public const int MaxBody = 256;

        private static readonly string[] Envelope = { "v", "type" };

        /// <summary>The validated state, or null with a reason.</summary>
        public static RingBootState Parse(Dictionary<string, object> obj, out string reason)
        {
            reason = null;
            object t;
            if (!obj.TryGetValue("type", out t) || !(t is string) || (string)t != Type) { reason = "request type not allowed"; return null; }
            object v;
            if (!obj.TryGetValue("v", out v) || !(v is int) || (int)v != 1) { reason = "unsupported version"; return null; }
            RingBootState s;
            return RingBootState.TryFromDictionary(obj, Envelope, out s, out reason) ? s : null;
        }
    }

    /// <summary>
    /// Applies pipe requests. Disk writes are coalesced: at most one per
    /// <see cref="WriteIntervalMs"/>; requests inside the window only remember the latest
    /// state, which is written when the window ends (the last request always wins).
    /// Unchanged states are not written. The log line follows the writes, so it is
    /// limited to the same window.
    /// </summary>
    internal sealed class RingBootRequestHandler
    {
        public const long WriteIntervalMs = 5000;

        private readonly RingBootStore _store;
        private readonly Action<string> _log;
        private readonly Func<long> _nowMs;
        private readonly Action<int, Action> _schedule;
        private readonly object _sync = new object();
        private bool _seeded;
        private RingBootState _last;     // what is known to be on disk; null if unknown
        private RingBootState _pending;  // latest request waiting for the window to end
        private long _lastWrite = long.MinValue;
        private bool _timerSet;

        public RingBootRequestHandler(RingBootStore store, Action<string> log, Func<long> nowMs = null, Action<int, Action> schedule = null)
        {
            _store = store;
            _log = log;
            _nowMs = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() * 1000 / System.Diagnostics.Stopwatch.Frequency);
            _schedule = schedule ?? ((ms, action) => { var t = new System.Threading.Timer(_ => action(), null, ms, System.Threading.Timeout.Infinite); });
        }

        /// <summary>Returns false only if an immediate write failed; a deferred write reports nothing back.</summary>
        public bool Handle(RingBootState state)
        {
            lock (_sync)
            {
                if (!_seeded)
                {
                    _seeded = true;
                    RingBootState loaded;
                    switch (_store.TryLoad(out loaded))
                    {
                        case RingBootStore.LoadStatus.Valid: _last = loaded; break;
                        case RingBootStore.LoadStatus.Absent: _last = RingBootState.None; break;
                        default: _last = null; break; // unreadable: the next request always writes
                    }
                }
                _pending = null;
                if (state.Equals(_last)) return true;
                long now = _nowMs();
                long wait = _lastWrite == long.MinValue ? 0 : _lastWrite + WriteIntervalMs - now;
                if (wait <= 0) return WriteLocked(state, now);
                _pending = state;
                if (!_timerSet)
                {
                    _timerSet = true;
                    _schedule((int)wait, Trailing);
                }
                return true;
            }
        }

        private void Trailing()
        {
            lock (_sync)
            {
                _timerSet = false;
                RingBootState s = _pending;
                _pending = null;
                if (s != null && !s.Equals(_last)) WriteLocked(s, _nowMs());
            }
        }

        private bool WriteLocked(RingBootState state, long now)
        {
            _lastWrite = now;
            if (!_store.Save(state))
            {
                _last = null; // unknown on disk
                return false;
            }
            _last = state;
            _log("ring boot state set to " + state);
            return true;
        }
    }
}
