// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using Microsoft.Win32.SafeHandles;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// The AMD FCH SMBus through the PawnIO SmbusPIIX4 module. The library's own
    /// PawnIO wrapper is not accessible from outside, so the module is driven
    /// directly through its documented IOCTLs, with error reporting.
    /// </summary>
    internal sealed class PawnIoSmbus : ISmbus, IDisposable
    {
        // IOCTL layout of the PawnIO driver.
        private const uint DeviceType = 41394u << 16;
        private const uint IoctlLoadBinary = DeviceType | 0x821 << 2;
        private const uint IoctlExecute = DeviceType | 0x841 << 2;
        private const int FnNameLength = 32;

        // Linux i2c-dev numbering, as the PawnIO SMBus modules expect.
        private const long Read = 1;
        private const long Write = 0;
        private const long ByteData = 2;
        private const long WordData = 3;
        private const long BlockData = 5;

        private const long Piix4Base = 0x0B00;

        private readonly SafeFileHandle _handle;

        private PawnIoSmbus(SafeFileHandle handle) => _handle = handle;

        /// <summary>Loads the module (a SmbusPIIX4.bin stream) and checks that the controller sits at the usual base.</summary>
        public static PawnIoSmbus Open(Func<Stream> module)
        {
            SafeFileHandle handle = CreateFile(@"\\?\GLOBALROOT\Device\PawnIO", 3, 3, IntPtr.Zero, 3, 0x80, IntPtr.Zero);
            if (handle.IsInvalid) throw new InvalidOperationException("PawnIO driver not available");
            byte[] bin;
            using (Stream s = module())
            using (var ms = new MemoryStream())
            {
                s.CopyTo(ms);
                bin = ms.ToArray();
            }
            uint read;
            if (!DeviceIoControl(handle, IoctlLoadBinary, bin, (uint)bin.Length, null, 0, out read, IntPtr.Zero))
            {
                int err = Marshal.GetLastWin32Error();
                handle.Dispose();
                throw new Win32Exception(err, "loading the SmbusPIIX4 module failed");
            }
            var bus = new PawnIoSmbus(handle);
            try
            {
                long[] id = bus.Execute("ioctl_identity", new long[0], 3);
                if (id.Length < 2 || id[1] != Piix4Base) throw new InvalidOperationException("unexpected SMBus controller");
            }
            catch
            {
                bus.Dispose();
                throw;
            }
            return bus;
        }

        public void Dispose() => _handle.Dispose();

        // Other tools may switch the PIIX4 port while they hold the mutex; only port 0 carries the RAM.
        public bool Ready()
        {
            long[] port = Execute("ioctl_piix4_port_sel", new long[] { -1 }, 1);
            return port.Length >= 1 && port[0] == 0;
        }

        public byte ReadByteData(byte addr, byte cmd)
        {
            long[] o = Execute("ioctl_smbus_xfer", new long[] { addr, Read, cmd, ByteData }, 1);
            if (o.Length < 1) throw new IOException("short SMBus read");
            return (byte)o[0];
        }

        public void WriteByteData(byte addr, byte cmd, byte value) =>
            Execute("ioctl_smbus_xfer", new long[] { addr, Write, cmd, ByteData, value }, 0);

        public void WriteWordData(byte addr, byte cmd, ushort value) =>
            Execute("ioctl_smbus_xfer", new long[] { addr, Write, cmd, WordData, value }, 0);

        // The block (length byte + data) goes in as five little-endian 64-bit words.
        public void WriteBlockData(byte addr, byte cmd, byte[] data)
        {
            if (data.Length < 1 || data.Length > 32) throw new ArgumentException("block of " + data.Length + " bytes");
            var raw = new byte[40];
            raw[0] = (byte)data.Length;
            Buffer.BlockCopy(data, 0, raw, 1, data.Length);
            var input = new long[9];
            input[0] = addr;
            input[1] = Write;
            input[2] = cmd;
            input[3] = BlockData;
            for (int i = 0; i < 5; i++) input[4 + i] = BitConverter.ToInt64(raw, i * 8);
            Execute("ioctl_smbus_xfer", input, 0);
        }

        private long[] Execute(string name, long[] input, int outLength)
        {
            var total = new byte[FnNameLength + input.Length * sizeof(long)];
            Encoding.ASCII.GetBytes(name, 0, Math.Min(name.Length, FnNameLength - 1), total, 0);
            Buffer.BlockCopy(input, 0, total, FnNameLength, input.Length * sizeof(long));
            var output = new byte[outLength * sizeof(long)];
            uint read;
            if (!DeviceIoControl(_handle, IoctlExecute, total, (uint)total.Length, output, (uint)output.Length, out read, IntPtr.Zero))
            {
                throw new Win32Exception(Marshal.GetLastWin32Error(), name + " failed");
            }
            var result = new long[read / sizeof(long)];
            Buffer.BlockCopy(output, 0, result, 0, result.Length * sizeof(long));
            return result;
        }

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern SafeFileHandle CreateFile(string fileName, uint access, uint share, IntPtr security, uint creation, uint flags, IntPtr template);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool DeviceIoControl(SafeFileHandle device, uint code, byte[] input, uint inputSize, byte[] output, uint outputSize, out uint returned, IntPtr overlapped);
    }

    /// <summary>
    /// Opens the PawnIO bus on first use and again after a failure (the driver or the
    /// SMBus controller may not be there at boot), at most every few seconds.
    /// </summary>
    internal sealed class LazySmbus : ISmbus, IDisposable
    {
        private readonly Func<ISmbus> _open;
        private readonly Action<string> _log;
        private readonly Func<long> _nowMs;
        private readonly long _retryMs;
        private ISmbus _bus;
        private long _retryAt;
        private bool _failureLogged;
        private bool _backoff;
        private int _failures;
        public const int FailuresBeforeReopen = 3;

        public LazySmbus(Func<ISmbus> open, Action<string> log, Func<long> nowMs = null, long retryMs = 5000)
        {
            _open = open;
            _log = log;
            _nowMs = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() * 1000 / System.Diagnostics.Stopwatch.Frequency);
            _retryMs = retryMs;
        }

        private ISmbus Bus
        {
            get
            {
                if (_bus != null) return _bus;
                if (_backoff && _nowMs() < _retryAt) throw new IOException("SMBus not available");
                try
                {
                    _bus = _open();
                    _backoff = false;
                    _failureLogged = false;
                    _log("SMBus bridge: PawnIO SmbusPIIX4 opened");
                    return _bus;
                }
                catch (Exception e)
                {
                    _retryAt = _nowMs() + _retryMs;
                    _backoff = true;
                    if (!_failureLogged) _log("SMBus bridge: opening PawnIO SmbusPIIX4 failed: " + e.Message);
                    _failureLogged = true;
                    throw new IOException("SMBus not available", e);
                }
            }
        }

        public bool Ready() => Call(b => b.Ready());

        public byte ReadByteData(byte addr, byte cmd) => Call(b => b.ReadByteData(addr, cmd));

        public void WriteByteData(byte addr, byte cmd, byte value) => Call(b => { b.WriteByteData(addr, cmd, value); return true; });

        public void WriteWordData(byte addr, byte cmd, ushort value) => Call(b => { b.WriteWordData(addr, cmd, value); return true; });

        public void WriteBlockData(byte addr, byte cmd, byte[] data) => Call(b => { b.WriteBlockData(addr, cmd, data); return true; });

        // After a transfer error the handle is closed and reopened (after a back-off),
        // in case the driver or the controller state is stale.
        private T Call<T>(Func<ISmbus, T> f)
        {
            ISmbus bus = Bus;
            try
            {
                T result = f(bus);
                _failures = 0;
                return result;
            }
            catch (Exception)
            {
                // A single NACK is normal (no stick at that address); a run of failures is not.
                if (++_failures >= FailuresBeforeReopen)
                {
                    (_bus as IDisposable)?.Dispose();
                    _bus = null;
                    _failures = 0;
                    _retryAt = _nowMs() + _retryMs;
                    _backoff = true;
                }
                throw;
            }
        }

        public void Dispose()
        {
            (_bus as IDisposable)?.Dispose();
            _bus = null;
        }
    }

    /// <summary>The named mutex "Global\Access_SMBUS.HTP.Method" that serializes SMBus users.</summary>
    internal sealed class SystemSmbusLock : ISmbusLock, IDisposable
    {
        public const string Name = @"Global\Access_SMBUS.HTP.Method";

        private readonly object _sync = new object();
        private Mutex _mutex;

        public bool TryAcquire(int timeoutMs)
        {
            lock (_sync)
            {
                if (_mutex == null) _mutex = new Mutex(false, Name);
            }
            try
            {
                return _mutex.WaitOne(timeoutMs);
            }
            catch (AbandonedMutexException)
            {
                return true; // the previous owner died; we own it now
            }
        }

        // Called on the thread that acquired it (one batch runs on one thread).
        public void Release() => _mutex.ReleaseMutex();

        public void Dispose() => _mutex?.Dispose();
    }
}
