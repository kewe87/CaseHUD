// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Linq;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.Cooling
{
    /// <summary>
    /// The fan headers an administrator released. The list is read again every few
    /// seconds, so a release or a withdrawal takes effect without restarting the service.
    /// </summary>
    internal sealed class CoolingAllowList
    {
        public const long CacheMs = 4000;

        private readonly Func<HashSet<string>> _read;
        private readonly Func<long> _now;
        private readonly Action<string> _log;
        private readonly object _sync = new object();
        private HashSet<string> _keys = new HashSet<string>();
        private long _readAt;
        private bool _have;
        private string _last = "";

        public CoolingAllowList(Func<HashSet<string>> read, Action<string> log, Func<long> nowMs = null)
        {
            _read = read;
            _log = log;
            _now = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() / (System.Diagnostics.Stopwatch.Frequency / 1000));
        }

        public static CoolingAllowList CreateDefault(Action<string> log)
        {
            var changes = new ChangeLog(log);
            return new CoolingAllowList(() => changes.Track(() => BridgeConfig.ReadCoolingHeaders(BridgeConfig.DefaultPath, changes.Write)), log);
        }

        public bool IsAllowed(string key)
        {
            lock (_sync)
            {
                long now = _now();
                if (!_have || now - _readAt >= CacheMs)
                {
                    HashSet<string> keys;
                    try { keys = _read() ?? new HashSet<string>(); }
                    catch (Exception) { keys = new HashSet<string>(); }
                    _keys = keys;
                    _readAt = now;
                    _have = true;
                    string joined = string.Join(", ", keys.OrderBy(k => k, StringComparer.Ordinal));
                    if (joined != _last)
                    {
                        _last = joined;
                        _log(joined.Length == 0 ? "cooling pipe: no fan header is released" : "cooling pipe: released fan headers: " + joined);
                    }
                }
                return _keys.Contains(key);
            }
        }
    }
}
