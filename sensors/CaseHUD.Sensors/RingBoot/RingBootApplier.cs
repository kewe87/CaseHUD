// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Threading;

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>An open ring. Dispose releases WinUSB at once (access is exclusive).</summary>
    internal interface IRingDevice : IDisposable
    {
        /// <summary>Sends one 64-byte packet on the bulk OUT endpoint.</summary>
        void Write(byte[] packet);
    }

    /// <summary>The USB side of the ring, replaced by a fake in tests.</summary>
    internal interface IRingUsb
    {
        /// <summary>Device paths of the ring currently present (may be empty while USB enumerates).</summary>
        IList<string> FindRings();

        /// <summary>Opens the ring exclusively. Throws if it is busy or gone.</summary>
        IRingDevice Open(string path);
    }

    internal static class SystemUptime
    {
        [DllImport("kernel32.dll")]
        private static extern ulong GetTickCount64();

        public static long Ms() => (long)GetTickCount64();
    }

    /// <summary>
    /// At service start: sends ONE frame (three 0x11 packets) to the ring and lets go.
    /// The handle is never kept, so the GUI can open the ring later. Nothing but 0x11
    /// packets built by <see cref="RingFrame"/> is ever written.
    /// </summary>
    internal sealed class RingBootApplier
    {
        public const int DefaultWindowMs = 30000;
        public const int DefaultIntervalMs = 2000;
        public const long MaxUptimeMs = 5 * 60 * 1000;

        private readonly IRingUsb _usb;
        private readonly Action<string> _log;
        private readonly Func<long> _nowMs;
        private readonly Action<int> _sleep;
        private readonly Func<long> _uptimeMs;

        public RingBootApplier(IRingUsb usb, Action<string> log, Func<long> nowMs = null, Action<int> sleep = null, Func<long> uptimeMs = null)
        {
            _usb = usb;
            _log = log;
            _nowMs = nowMs ?? (() => Environment.TickCount);
            _sleep = sleep ?? (ms => Thread.Sleep(ms));
            _uptimeMs = uptimeMs ?? SystemUptime.Ms;
        }

        /// <summary>Returns true if the frame was written. Logs exactly one line with the outcome.</summary>
        public bool Apply(RingBootState state, CancellationToken ct, int windowMs = DefaultWindowMs, int intervalMs = DefaultIntervalMs)
        {
            if (state == null || state.Mode == RingBootMode.None)
            {
                _log("ring boot: mode none, ring left alone");
                return false;
            }
            // A service restart in the middle of a session must not overwrite what the GUI shows.
            if (_uptimeMs() > MaxUptimeMs)
            {
                _log("ring boot: skipped, not at boot");
                return false;
            }
            byte[][] frame = RingFrame.Build(state);
            long start = _nowMs();
            string last = "ring not found";
            int attempts = 0;
            while (true)
            {
                attempts++;
                try
                {
                    IList<string> rings = _usb.FindRings();
                    if (rings != null && rings.Count > 0)
                    {
                        using (IRingDevice dev = _usb.Open(rings[0]))
                        {
                            foreach (byte[] pkt in frame) dev.Write(pkt);
                        }
                        _log("ring boot: applied " + state + " (attempt " + attempts + ")");
                        return true;
                    }
                    last = "ring not found";
                }
                catch (Exception e)
                {
                    last = e.Message;
                }
                if (ct.IsCancellationRequested) { _log("ring boot: cancelled (" + last + ")"); return false; }
                if (_nowMs() - start + intervalMs > windowMs) break;
                _sleep(intervalMs);
                if (ct.IsCancellationRequested) { _log("ring boot: cancelled (" + last + ")"); return false; }
            }
            _log("ring boot: gave up after " + attempts + " attempt(s): " + last);
            return false;
        }
    }
}
