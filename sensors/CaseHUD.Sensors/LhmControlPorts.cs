// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Threading;
using CaseHUD.Sensors.Cooling;
using LibreHardwareMonitor.Hardware;

namespace CaseHUD.Sensors
{
    /// <summary>
    /// The fan headers of the mainboard's Super I/O chip. Graphics cards, liquid coolers and
    /// other devices have controls too; they cannot be reached this way.
    /// </summary>
    internal sealed class LhmControlPorts : IControlPorts
    {
        // Everything on this bus waits for this mutex. The library gives up after a few
        // milliseconds and writes nothing, without saying so. It takes the mutex again on
        // the same thread without waiting, so holding it here makes all of its writes land.
        private const string IsaBusMutex = @"Global\Access_ISABUS.HTP.Method";
        private const int SetWaitMs = 300; // the client waits 500 ms for its answer and asks again next second
        private const int ReleaseWaitMs = 2000;

        private readonly Func<IEnumerable<IHardware>> _hardware;
        private readonly Action<string> _log;
        private readonly Dictionary<string, long> _writtenAt = new Dictionary<string, long>();
        private Mutex _bus;

        /// <param name="hardware">The opened hardware; null while it is still being opened.</param>
        /// <param name="log">Where a surprise on the bus is told.</param>
        public LhmControlPorts(Func<IEnumerable<IHardware>> hardware, Action<string> log)
        {
            _hardware = hardware;
            _log = log;
        }

        public bool Set(string key, int percent)
        {
            IControl control = Find(key);
            if (control == null) return false;
            return OnBus(SetWaitMs, () =>
            {
                long now = NowMs();
                if (control.ControlMode != ControlMode.Software || control.SoftwareValue != percent)
                {
                    control.SetSoftware(percent);
                    _writtenAt[key] = now;
                    return;
                }
                long at;
                _writtenAt.TryGetValue(key, out at);
                if (!ControlValues.MustRewrite(percent, control.Sensor.Value, now - at)) return;
                _log("cooling pipe: " + key + " runs at " + control.Sensor.Value + " % instead of " + percent + " %, setting it again");
                control.SetSoftware(percent == 100 ? 99 : percent + 1); // the same value would not reach the chip
                control.SetSoftware(percent);
                _writtenAt[key] = now;
            });
        }

        public bool Release(string key)
        {
            IControl control = Find(key);
            if (control == null) return false;
            return OnBus(ReleaseWaitMs, control.SetDefault);
        }

        private bool OnBus(int waitMs, Action write)
        {
            if (_bus == null) _bus = new Mutex(false, IsaBusMutex);
            bool mine;
            try
            {
                mine = _bus.WaitOne(waitMs);
            }
            catch (AbandonedMutexException)
            {
                mine = true; // its owner died; it is ours now
            }
            if (!mine) return false;
            try
            {
                write();
                return true;
            }
            finally
            {
                _bus.ReleaseMutex();
            }
        }

        private static long NowMs() => System.Diagnostics.Stopwatch.GetTimestamp() / (System.Diagnostics.Stopwatch.Frequency / 1000);

        private IControl Find(string key)
        {
            IEnumerable<IHardware> all = _hardware();
            if (all == null) return null;
            foreach (IHardware hw in all)
            {
                IControl found = Find(hw, key);
                if (found != null) return found;
            }
            return null;
        }

        private static IControl Find(IHardware hw, string key)
        {
            if (hw.HardwareType == HardwareType.SuperIO)
            {
                foreach (ISensor sensor in hw.Sensors)
                {
                    if (sensor.SensorType == SensorType.Control && sensor.Control != null && sensor.Identifier.ToString() == key)
                    {
                        return sensor.Control;
                    }
                }
            }
            foreach (IHardware sub in hw.SubHardware)
            {
                IControl found = Find(sub, key);
                if (found != null) return found;
            }
            return null;
        }
    }

    /// <summary>Nothing is remembered between starts, except that a header's value starts at full speed.</summary>
    internal sealed class HeaderStartSettings : ISettings
    {
        public bool Contains(string name) => false;

        public void SetValue(string name, string value) { }

        public string GetValue(string name, string value) => ControlValues.Start(name, value);

        public void Remove(string name) { }
    }
}
