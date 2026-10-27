// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>The few SMBus transfers the bridge needs (real: PawnIO SmbusPIIX4, tests: a fake).</summary>
    internal interface ISmbus
    {
        /// <summary>
        /// Called once per batch while the bus mutex is held. False if the bus
        /// cannot be used right now (another tool selected a different port).
        /// </summary>
        bool Ready();

        byte ReadByteData(byte addr, byte cmd);

        void WriteByteData(byte addr, byte cmd, byte value);

        void WriteWordData(byte addr, byte cmd, ushort value);

        void WriteBlockData(byte addr, byte cmd, byte[] data);
    }

    /// <summary>The system-wide SMBus mutex shared with LibreHardwareMonitor and other tools.</summary>
    internal interface ISmbusLock
    {
        /// <summary>Waits at most <paramref name="timeoutMs"/>; false if the mutex is held elsewhere.</summary>
        bool TryAcquire(int timeoutMs);

        void Release();
    }

    /// <summary>A named pipe (or a test double) that serves bridge connections until disposed.</summary>
    internal interface IPipeServer : IDisposable
    {
        void Start();

        /// <summary>False once the accept loop has died, so the host can restart the pipe.</summary>
        bool IsAlive { get; }
    }
}
