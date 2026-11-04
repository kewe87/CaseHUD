// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.IO;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Text;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>
    /// The ring boot state file (%ProgramData%\CaseHUD\ring-boot.json). The service is the
    /// only writer. Reading applies the same trust rules as sensors.json: the directory
    /// and file must not be reparse points and must only be modifiable by SYSTEM and
    /// administrators, otherwise the state counts as "none".
    /// </summary>
    internal sealed class RingBootStore
    {
        public static readonly string DefaultPath = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "CaseHUD", "ring-boot.json");

        private readonly string _path;
        private readonly Action<string> _log;
        private readonly Func<FileSecurity, bool> _fileTrusted;
        private readonly Func<DirectorySecurity, bool> _dirTrusted;
        private readonly object _sync = new object();

        public RingBootStore(string path, Action<string> log, Func<FileSecurity, bool> fileTrusted = null, Func<DirectorySecurity, bool> dirTrusted = null)
        {
            _path = path;
            _log = log;
            _fileTrusted = fileTrusted ?? BridgeConfig.IsTrusted;
            _dirTrusted = dirTrusted ?? BridgeConfig.IsDirectoryTrusted;
        }

        public enum LoadStatus
        {
            Absent,      // no file: nothing stored yet
            Valid,       // a trusted, well-formed file
            Unreadable,  // present but untrusted, malformed or unreadable (state counts as none)
        }

        public RingBootState Load()
        {
            RingBootState state;
            TryLoad(out state);
            return state;
        }

        /// <summary>Tri-state load; <paramref name="state"/> is None unless the status is Valid.</summary>
        public LoadStatus TryLoad(out RingBootState state)
        {
            state = RingBootState.None;
            try
            {
                string dir = Path.GetDirectoryName(_path);
                var di = new DirectoryInfo(dir);
                if (!di.Exists || !File.Exists(_path)) return LoadStatus.Absent;
                if ((di.Attributes & FileAttributes.ReparsePoint) != 0 || (new FileInfo(_path).Attributes & FileAttributes.ReparsePoint) != 0)
                {
                    _log("ring boot: ignoring " + _path + " (reparse point)");
                    return LoadStatus.Unreadable;
                }
                if (!_dirTrusted(di.GetAccessControl()))
                {
                    _log("ring boot: ignoring " + _path + " (" + dir + " is not restricted to administrators)");
                    return LoadStatus.Unreadable;
                }
                using (var fs = new FileStream(_path, FileMode.Open, FileAccess.Read, FileShare.Read))
                {
                    if (!_fileTrusted(fs.GetAccessControl()))
                    {
                        _log("ring boot: ignoring " + _path + " (not owned by an administrator or writable by others)");
                        return LoadStatus.Unreadable;
                    }
                    if (fs.Length == 0 || fs.Length > RingBootState.MaxJsonBytes) return LoadStatus.Unreadable;
                    var buf = new byte[fs.Length];
                    int got = 0;
                    while (got < buf.Length)
                    {
                        int r = fs.Read(buf, got, buf.Length - got);
                        if (r <= 0) return LoadStatus.Unreadable;
                        got += r;
                    }
                    RingBootState parsed;
                    if (!RingBootState.TryParseFile(buf, out parsed)) return LoadStatus.Unreadable;
                    state = parsed;
                    return LoadStatus.Valid;
                }
            }
            catch (Exception e)
            {
                _log("ring boot: reading " + _path + " failed: " + e.Message);
                return LoadStatus.Unreadable;
            }
        }

        /// <summary>
        /// Writes the state atomically. The directory is checked like on load (no reparse
        /// point, administrators only); the temp file has a random name and is created
        /// with CreateNew in the same directory, then renamed. Returns false on failure.
        /// </summary>
        public bool Save(RingBootState state)
        {
            lock (_sync)
            {
                string tmp = _path + "." + Guid.NewGuid().ToString("N") + ".tmp";
                try
                {
                    string dir = Path.GetDirectoryName(_path);
                    EnsureDirectory(dir);
                    var di = new DirectoryInfo(dir);
                    if ((di.Attributes & FileAttributes.ReparsePoint) != 0)
                    {
                        _log("ring boot: not saving, " + dir + " is a reparse point");
                        return false;
                    }
                    if (!_dirTrusted(di.GetAccessControl()))
                    {
                        _log("ring boot: not saving, " + dir + " is not restricted to administrators");
                        return false;
                    }
                    byte[] data = new UTF8Encoding(false).GetBytes(state.ToJson());
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
                    _log("ring boot: saving failed: " + e.Message);
                    try { File.Delete(tmp); } catch { }
                    return false;
                }
            }
        }

        /// <summary>
        /// Creates the directory with the same ACL the installer sets: no inheritance,
        /// SYSTEM and Administrators full control, Users read/execute only.
        /// An existing directory is left as it is (the trust check on load covers it).
        /// </summary>
        internal static void EnsureDirectory(string dir)
        {
            if (Directory.Exists(dir)) return;
            Directory.CreateDirectory(dir, CreateSecurity());
        }

        internal static DirectorySecurity CreateSecurity()
        {
            var sec = new DirectorySecurity();
            sec.SetAccessRuleProtection(true, false);
            const InheritanceFlags inh = InheritanceFlags.ContainerInherit | InheritanceFlags.ObjectInherit;
            sec.AddAccessRule(new FileSystemAccessRule(new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null), FileSystemRights.FullControl, inh, PropagationFlags.None, AccessControlType.Allow));
            sec.AddAccessRule(new FileSystemAccessRule(new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null), FileSystemRights.FullControl, inh, PropagationFlags.None, AccessControlType.Allow));
            sec.AddAccessRule(new FileSystemAccessRule(new SecurityIdentifier(WellKnownSidType.BuiltinUsersSid, null), FileSystemRights.ReadAndExecute, inh, PropagationFlags.None, AccessControlType.Allow));
            return sec;
        }
    }
}
