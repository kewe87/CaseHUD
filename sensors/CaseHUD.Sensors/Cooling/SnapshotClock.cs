// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;

namespace CaseHUD.Sensors.Cooling
{
    /// <summary>
    /// Tells how old the sensor data is. The JSON of the service is built once a second from
    /// the last readings and served as it is, so if reading the hardware hangs or fails, the
    /// answer goes on looking like a valid one. The age of the snapshot, and of each hardware
    /// in it, lets the client tell.
    /// </summary>
    internal sealed class SnapshotClock
    {
        /// <summary>The header with the age of the snapshot, in milliseconds.</summary>
        public const string AgeHeader = "X-Snapshot-Age-Ms";

        private readonly Func<long> _now;
        private readonly object _sync = new object();
        private readonly Dictionary<string, long> _updatedAt = new Dictionary<string, long>(StringComparer.Ordinal);
        private long _builtAt = -1;

        public SnapshotClock(Func<long> nowMs = null)
        {
            _now = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() / (System.Diagnostics.Stopwatch.Frequency / 1000));
        }

        /// <summary>The values of this hardware were just read.</summary>
        public void Updated(string hardwareId)
        {
            lock (_sync) _updatedAt[hardwareId] = _now();
        }

        /// <summary>A snapshot was just completed and is what the next answer carries.</summary>
        public void Built()
        {
            lock (_sync) _builtAt = _now();
        }

        /// <summary>How long ago the snapshot was completed; null before the first.</summary>
        public long? SnapshotAgeMs()
        {
            lock (_sync) return _builtAt < 0 ? (long?)null : Math.Max(_now() - _builtAt, 0);
        }

        /// <summary>
        /// How long before now the values of this hardware were read; null if they never were. For
        /// a snapshot that is being built, this is the age they have in it.
        /// </summary>
        public long? HardwareAgeMs(string hardwareId)
        {
            lock (_sync)
            {
                long at;
                return _updatedAt.TryGetValue(hardwareId, out at) ? Math.Max(_now() - at, 0) : (long?)null;
            }
        }
    }
}
