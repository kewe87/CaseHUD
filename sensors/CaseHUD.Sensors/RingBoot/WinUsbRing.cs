// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>
    /// Real USB access to the 8.8" ring: device interface lookup by GUID (present
    /// interfaces, filtered by VID/PID), CreateFile, WinUsb_Initialize, bulk OUT 0x01.
    /// Only RingFrame packets are accepted by Write.
    /// </summary>
    internal sealed class WinUsbRingUsb : IRingUsb
    {
        public static readonly Guid InterfaceGuid = new Guid("1D4B2365-4749-48EA-B38A-7C6FDDDD7E26");
        public const string IdFilter = "vid_0416&pid_8050";
        public const byte PipeOut = 0x01;
        public const int TimeoutMs = 500;

        public IList<string> FindRings()
        {
            var result = new List<string>();
            Guid g = InterfaceGuid;
            uint len;
            int cr = CM_Get_Device_Interface_List_SizeW(out len, ref g, null, 0);
            if (cr != 0) throw new Win32Exception("device interface list size failed (CR " + cr + ")");
            if (len <= 1) return result;
            var buf = new char[len];
            cr = CM_Get_Device_Interface_ListW(ref g, null, buf, len, 0);
            if (cr != 0) throw new Win32Exception("device interface list failed (CR " + cr + ")");
            foreach (string p in new string(buf).Split('\0'))
            {
                if (p.Length > 0 && p.IndexOf(IdFilter, StringComparison.OrdinalIgnoreCase) >= 0) result.Add(p);
            }
            return result;
        }

        public IRingDevice Open(string path)
        {
            const uint GenericRead = 0x80000000, GenericWrite = 0x40000000;
            const uint ShareRead = 1, ShareWrite = 2, OpenExisting = 3, Overlapped = 0x40000000;
            SafeFileHandle h = CreateFileW(path, GenericRead | GenericWrite, ShareRead | ShareWrite, IntPtr.Zero, OpenExisting, Overlapped, IntPtr.Zero);
            if (h.IsInvalid) throw new Win32Exception(Marshal.GetLastWin32Error(), "opening the ring failed");
            IntPtr ih;
            if (!WinUsb_Initialize(h, out ih))
            {
                int err = Marshal.GetLastWin32Error();
                h.Dispose();
                throw new Win32Exception(err, "WinUsb_Initialize failed");
            }
            uint ms = TimeoutMs;
            WinUsb_SetPipePolicy(ih, PipeOut, 0x03, 4, ref ms);
            return new Device(h, ih);
        }

        private sealed class Device : IRingDevice
        {
            private SafeFileHandle _h;
            private IntPtr _ih;

            public Device(SafeFileHandle h, IntPtr ih) { _h = h; _ih = ih; }

            public void Write(byte[] packet)
            {
                if (!RingFrame.IsValidPacket(packet)) throw new ArgumentException("not a ring LED packet");
                if (_ih == IntPtr.Zero) throw new ObjectDisposedException("ring");
                uint n;
                if (!WinUsb_WritePipe(_ih, PipeOut, packet, (uint)packet.Length, out n, IntPtr.Zero))
                    throw new Win32Exception(Marshal.GetLastWin32Error(), "writing to the ring failed");
                if (n != packet.Length) throw new System.IO.IOException("short write to the ring");
            }

            public void Dispose()
            {
                if (_ih != IntPtr.Zero) { WinUsb_Free(_ih); _ih = IntPtr.Zero; }
                if (_h != null) { _h.Dispose(); _h = null; }
            }
        }

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern int CM_Get_Device_Interface_List_SizeW(out uint length, ref Guid guid, string deviceId, uint flags);

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern int CM_Get_Device_Interface_ListW(ref Guid guid, string deviceId, [Out] char[] buffer, uint length, uint flags);

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern SafeFileHandle CreateFileW(string name, uint access, uint share, IntPtr security, uint disposition, uint flags, IntPtr template);

        [DllImport("winusb.dll", SetLastError = true)]
        private static extern bool WinUsb_Initialize(SafeFileHandle device, out IntPtr handle);

        [DllImport("winusb.dll", SetLastError = true)]
        private static extern bool WinUsb_Free(IntPtr handle);

        [DllImport("winusb.dll", SetLastError = true)]
        private static extern bool WinUsb_SetPipePolicy(IntPtr handle, byte pipe, uint policy, uint valueLength, ref uint value);

        [DllImport("winusb.dll", SetLastError = true)]
        private static extern bool WinUsb_WritePipe(IntPtr handle, byte pipe, byte[] buffer, uint length, out uint transferred, IntPtr overlapped);
    }
}
