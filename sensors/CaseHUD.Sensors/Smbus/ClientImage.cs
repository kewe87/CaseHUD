// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Tells which program a pipe client is, by the image file of its process. The pipe
    /// ACL admits every interactive user; this is what keeps a program of another user
    /// from using a pipe that only CaseHUD is meant to speak to.
    /// </summary>
    internal static class ClientImage
    {
        /// <summary>The programs of an installation that may use the cooling pipe.</summary>
        public static readonly string[] CoolingClients = { "CaseHUD.exe", "CaseHUD-Guard.exe" };

        /// <summary>
        /// The check for the service of this installation: the service runs from
        /// {installDir}\sensors, and its clients are the named programs in {installDir}.
        /// A debug build, which is never installed, takes every client.
        /// </summary>
        public static Func<int, bool> ForThisService(string[] names)
        {
#if DEBUG
            return _ => true;
#else
            string exe = System.Reflection.Assembly.GetExecutingAssembly().Location;
            return ForInstall(Path.GetDirectoryName(Path.GetDirectoryName(exe)), names, ImageOf);
#endif
        }

        /// <summary>
        /// Accepts a process whose image is one of names directly in installDir. A process
        /// whose image cannot be read is refused.
        /// </summary>
        public static Func<int, bool> ForInstall(string installDir, string[] names, Func<int, string> imageOf)
        {
            var allowed = new System.Collections.Generic.HashSet<string>(StringComparer.OrdinalIgnoreCase);
            if (!string.IsNullOrEmpty(installDir))
            {
                foreach (string n in names) allowed.Add(Path.Combine(Path.GetFullPath(installDir), n));
            }
            return pid =>
            {
                if (pid <= 0 || allowed.Count == 0) return false;
                string image;
                try { image = imageOf(pid); }
                catch (Exception) { return false; }
                if (string.IsNullOrEmpty(image)) return false;
                try { return allowed.Contains(Path.GetFullPath(image)); }
                catch (Exception) { return false; }
            };
        }

        /// <summary>The image file of a process; null if it cannot be read.</summary>
        public static string ImageOf(int pid)
        {
            const uint QueryLimited = 0x1000;
            IntPtr h = OpenProcess(QueryLimited, false, (uint)pid);
            if (h == IntPtr.Zero) return null;
            try
            {
                var sb = new StringBuilder(1024);
                int size = sb.Capacity;
                return QueryFullProcessImageName(h, 0, sb, ref size) ? sb.ToString(0, size) : null;
            }
            finally
            {
                CloseHandle(h);
            }
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr OpenProcess(uint access, bool inherit, uint pid);

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern bool QueryFullProcessImageName(IntPtr process, int flags, StringBuilder name, ref int size);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool CloseHandle(IntPtr handle);
    }
}
