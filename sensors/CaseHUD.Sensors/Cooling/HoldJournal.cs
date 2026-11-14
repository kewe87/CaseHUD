// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Web.Script.Serialization;
using CaseHUD.Sensors.RingBoot;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.Cooling
{
    /// <summary>
    /// What the service has set on the mainboard and not given back. A header stays at the
    /// duty the service wrote when the service dies, so the next start must know which
    /// headers that concerns. It cannot be found out from the board.
    /// </summary>
    internal sealed class HoldRecord
    {
        /// <summary>Headers written by this service and not yet given back to the board.</summary>
        public readonly HashSet<string> Held = new HashSet<string>(StringComparer.Ordinal);

        /// <summary>
        /// Headers the board cannot be given back (see ControlHolds): they sit at full speed
        /// until the PC restarts.
        /// </summary>
        public readonly HashSet<string> Parked = new HashSet<string>(StringComparer.Ordinal);

        /// <summary>Milliseconds since the PC started, when this was written.</summary>
        public long UptimeMs;

        public bool IsEmpty => Held.Count == 0 && Parked.Count == 0;
    }

    internal interface IHoldJournal
    {
        /// <summary>The last record, or null if there is none or it cannot be trusted.</summary>
        HoldRecord Load();

        /// <summary>True only if the record is on disk when this returns.</summary>
        bool Save(HoldRecord record);
    }

    /// <summary>Time since the PC started; a restart sets it back.</summary>
    internal static class Uptime
    {
        [DllImport("kernel32.dll")]
        private static extern ulong GetTickCount64();

        public static long Ms() => (long)GetTickCount64();
    }

    /// <summary>
    /// The journal file (%ProgramData%\CaseHUD\cooling-holds.json). Like ring-boot.json it is
    /// written by the service alone and only read if the folder and the file belong to
    /// administrators; anything else counts as "nothing held".
    /// </summary>
    internal sealed class FileHoldJournal : IHoldJournal
    {
        public static readonly string DefaultPath = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "CaseHUD", "cooling-holds.json");

        public const int MaxBytes = 16 * 1024;
        private const int MaxKeys = 64;

        private readonly string _path;
        private readonly Action<string> _log;
        private readonly Func<System.Security.AccessControl.FileSecurity, bool> _fileTrusted;
        private readonly Func<System.Security.AccessControl.DirectorySecurity, bool> _dirTrusted;
        private readonly Func<long> _uptime;
        private readonly object _sync = new object();

        public FileHoldJournal(string path, Action<string> log,
            Func<System.Security.AccessControl.FileSecurity, bool> fileTrusted = null,
            Func<System.Security.AccessControl.DirectorySecurity, bool> dirTrusted = null,
            Func<long> uptimeMs = null)
        {
            _path = path;
            _log = log;
            _fileTrusted = fileTrusted ?? BridgeConfig.IsTrusted;
            _dirTrusted = dirTrusted ?? BridgeConfig.IsDirectoryTrusted;
            _uptime = uptimeMs ?? Uptime.Ms;
        }

        public HoldRecord Load()
        {
            lock (_sync)
            {
                try
                {
                    string dir = Path.GetDirectoryName(_path);
                    var di = new DirectoryInfo(dir);
                    if (!di.Exists || !File.Exists(_path)) return null;
                    if ((di.Attributes & FileAttributes.ReparsePoint) != 0 || (new FileInfo(_path).Attributes & FileAttributes.ReparsePoint) != 0)
                    {
                        _log("cooling holds: ignoring " + _path + " (reparse point)");
                        return null;
                    }
                    if (!_dirTrusted(di.GetAccessControl()))
                    {
                        _log("cooling holds: ignoring " + _path + " (" + dir + " is not restricted to administrators)");
                        return null;
                    }
                    byte[] buf;
                    using (var fs = new FileStream(_path, FileMode.Open, FileAccess.Read, FileShare.Read))
                    {
                        if (!_fileTrusted(fs.GetAccessControl()))
                        {
                            _log("cooling holds: ignoring " + _path + " (not owned by an administrator or writable by others)");
                            return null;
                        }
                        if (fs.Length == 0 || fs.Length > MaxBytes) return null;
                        buf = new byte[fs.Length];
                        int got = 0;
                        while (got < buf.Length)
                        {
                            int r = fs.Read(buf, got, buf.Length - got);
                            if (r <= 0) return null;
                            got += r;
                        }
                    }
                    return Parse(Encoding.UTF8.GetString(buf));
                }
                catch (Exception e)
                {
                    _log("cooling holds: reading " + _path + " failed: " + e.Message);
                    return null;
                }
            }
        }

        internal static HoldRecord Parse(string json)
        {
            var root = new JavaScriptSerializer().DeserializeObject(json) as Dictionary<string, object>;
            object v;
            if (root == null || !root.TryGetValue("v", out v) || !(v is int) || (int)v != 1) return null;
            var rec = new HoldRecord();
            object up;
            if (root.TryGetValue("uptimeMs", out up) && (up is int || up is long)) rec.UptimeMs = Convert.ToInt64(up);
            Fill(root, "held", rec.Held);
            Fill(root, "parked", rec.Parked);
            return rec;
        }

        private static void Fill(Dictionary<string, object> root, string name, HashSet<string> into)
        {
            object o;
            var list = root.TryGetValue(name, out o) ? o as System.Collections.IEnumerable : null;
            if (list == null || list is string) return;
            foreach (object item in list)
            {
                string key = item as string;
                // Only what the cooling pipe could have held.
                if (key != null && CoolingConnection.KeyPattern.IsMatch(key) && into.Count < MaxKeys) into.Add(key);
            }
        }

        public bool Save(HoldRecord record)
        {
            lock (_sync)
            {
                string tmp = _path + "." + Guid.NewGuid().ToString("N") + ".tmp";
                try
                {
                    string dir = Path.GetDirectoryName(_path);
                    RingBootStore.EnsureDirectory(dir);
                    var di = new DirectoryInfo(dir);
                    if ((di.Attributes & FileAttributes.ReparsePoint) != 0)
                    {
                        _log("cooling holds: not saving, " + dir + " is a reparse point");
                        return false;
                    }
                    if (!_dirTrusted(di.GetAccessControl()))
                    {
                        _log("cooling holds: not saving, " + dir + " is not restricted to administrators");
                        return false;
                    }
                    if (File.Exists(_path) && (new FileInfo(_path).Attributes & FileAttributes.ReparsePoint) != 0)
                    {
                        _log("cooling holds: not saving, " + _path + " is a reparse point");
                        return false;
                    }
                    record.UptimeMs = _uptime();
                    byte[] data = new UTF8Encoding(false).GetBytes(ToJson(record));
                    using (var fs = new FileStream(tmp, FileMode.CreateNew, FileAccess.Write, FileShare.None))
                    {
                        if ((new FileInfo(tmp).Attributes & FileAttributes.ReparsePoint) != 0)
                            throw new IOException("temp file is a reparse point");
                        fs.Write(data, 0, data.Length);
                        fs.Flush(true);
                    }
                    if (File.Exists(_path)) File.Replace(tmp, _path, null);
                    else File.Move(tmp, _path);
                    return true;
                }
                catch (Exception e)
                {
                    _log("cooling holds: saving failed: " + e.Message);
                    try { File.Delete(tmp); } catch { }
                    return false;
                }
            }
        }

        internal static string ToJson(HoldRecord r)
        {
            var obj = new Dictionary<string, object>
            {
                { "v", 1 },
                { "uptimeMs", r.UptimeMs },
                { "held", new List<string>(r.Held) },
                { "parked", new List<string>(r.Parked) },
            };
            return new JavaScriptSerializer().Serialize(obj);
        }
    }
}
