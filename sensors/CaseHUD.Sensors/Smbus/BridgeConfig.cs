// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Web.Script.Serialization;
using CaseHUD.Sensors.Cooling;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Reads the service settings (C:\ProgramData\CaseHUD\sensors.json). Keys:
    /// "smbusBridge" (default false) and "coolingHeaders" (default none). ProgramData\CaseHUD is locked down by the
    /// installer (users read-only), but on older installs or portable setups normal users
    /// may still add files, so a file is only honored when it
    /// is owned by an administrator account and nobody else can write to it; anything
    /// else counts as "off".
    /// </summary>
    internal static class BridgeConfig
    {
        public static readonly string DefaultPath = System.IO.Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "CaseHUD", "sensors.json");

        private static readonly HashSet<string> TrustedSids = new HashSet<string>
        {
            "S-1-5-18",      // SYSTEM
            "S-1-5-32-544",  // Administrators
            "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464", // TrustedInstaller
        };

        private const FileSystemRights WriteMask =
            FileSystemRights.WriteData | FileSystemRights.AppendData | FileSystemRights.WriteExtendedAttributes |
            FileSystemRights.WriteAttributes | FileSystemRights.Delete | FileSystemRights.ChangePermissions |
            FileSystemRights.TakeOwnership;

        private const FileSystemRights DirReplaceMask =
            FileSystemRights.DeleteSubdirectoriesAndFiles | FileSystemRights.Delete |
            FileSystemRights.ChangePermissions | FileSystemRights.TakeOwnership;

        /// <summary>True only if the file exists, is trusted and contains <c>"smbusBridge": true</c>.</summary>
        public static bool ReadSmbusBridge(string path, Action<string> log, Func<FileSecurity, bool> trusted = null, Func<DirectorySecurity, bool> dirTrusted = null)
        {
            Dictionary<string, object> obj = ReadTrusted(path, "SMBus bridge", log, trusted, dirTrusted);
            object v;
            return obj != null && obj.TryGetValue("smbusBridge", out v) && v is bool && (bool)v;
        }

        /// <summary>
        /// The mainboard fan headers an administrator released for the cooling pipe
        /// (<c>"coolingHeaders": ["/lpc/nct6701d/0/control/6"]</c>). Nothing is released
        /// when the key or the file is missing or the file is not trusted.
        /// </summary>
        public static HashSet<string> ReadCoolingHeaders(string path, Action<string> log, Func<FileSecurity, bool> trusted = null, Func<DirectorySecurity, bool> dirTrusted = null)
        {
            var keys = new HashSet<string>(StringComparer.Ordinal);
            Dictionary<string, object> obj = ReadTrusted(path, "cooling pipe", log, trusted, dirTrusted);
            object v;
            var list = obj != null && obj.TryGetValue("coolingHeaders", out v) ? v as System.Collections.IEnumerable : null;
            if (list == null || list is string) return keys;
            foreach (object item in list)
            {
                string key = item as string;
                if (key != null && CoolingConnection.KeyPattern.IsMatch(key)) keys.Add(key);
                else log("cooling pipe: ignoring an entry in coolingHeaders that is not a fan header of the mainboard");
            }
            return keys;
        }

        /// <summary>
        /// The parsed settings file, or null when it is absent or not trusted. The directory
        /// must be an ordinary directory (no junction) that only administrators can modify or
        /// take over; the file must not be a symbolic link either.
        /// </summary>
        private static Dictionary<string, object> ReadTrusted(string path, string prefix, Action<string> log, Func<FileSecurity, bool> trusted, Func<DirectorySecurity, bool> dirTrusted)
        {
            try
            {
                string dir = System.IO.Path.GetDirectoryName(path);
                var di = new DirectoryInfo(dir);
                if (!di.Exists) return null;
                if ((di.Attributes & FileAttributes.ReparsePoint) != 0)
                {
                    log(prefix + ": ignoring " + path + " (" + dir + " is a reparse point)");
                    return null;
                }
                if (!File.Exists(path)) return null;
                if ((new FileInfo(path).Attributes & FileAttributes.ReparsePoint) != 0)
                {
                    log(prefix + ": ignoring " + path + " (the file is a reparse point)");
                    return null;
                }
                if (!(dirTrusted ?? IsDirectoryTrusted)(di.GetAccessControl()))
                {
                    log(prefix + ": ignoring " + path + " (" + dir + " must be owned by an administrator and not allow other users to delete or replace files in it)");
                    return null;
                }
                using (var fs = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read))
                {
                    if (!(trusted ?? IsTrusted)(fs.GetAccessControl()))
                    {
                        log(prefix + ": ignoring " + path + " (must be owned by an administrator and not writable by other users)");
                        return null;
                    }
                    if (fs.Length > 64 * 1024) return null;
                    string json = new StreamReader(fs).ReadToEnd();
                    return new JavaScriptSerializer().DeserializeObject(json) as Dictionary<string, object>;
                }
            }
            catch (FileNotFoundException) { return null; }
            catch (DirectoryNotFoundException) { return null; }
            catch (Exception e)
            {
                log(prefix + ": reading " + path + " failed: " + e.Message);
                return null;
            }
        }

        /// <summary>The directory is owned by an administrator and nobody else may delete files in it, change its permissions or take it over.</summary>
        public static bool IsDirectoryTrusted(DirectorySecurity sec)
        {
            IdentityReference owner = sec.GetOwner(typeof(SecurityIdentifier));
            if (owner == null || !TrustedSids.Contains(owner.Value)) return false;
            foreach (FileSystemAccessRule r in sec.GetAccessRules(true, true, typeof(SecurityIdentifier)))
            {
                if (r.AccessControlType != AccessControlType.Allow) continue;
                if ((r.FileSystemRights & DirReplaceMask) == 0) continue;
                string sid = r.IdentityReference.Value;
                if (!TrustedSids.Contains(sid) && sid != "S-1-3-0") return false;
            }
            return true;
        }

        /// <summary>The owner is SYSTEM/Administrators and only they (and CREATOR OWNER) hold write access.</summary>
        public static bool IsTrusted(FileSecurity sec)
        {
            IdentityReference owner = sec.GetOwner(typeof(SecurityIdentifier));
            if (owner == null || !TrustedSids.Contains(owner.Value)) return false;
            foreach (FileSystemAccessRule r in sec.GetAccessRules(true, true, typeof(SecurityIdentifier)))
            {
                if (r.AccessControlType != AccessControlType.Allow) continue;
                if ((r.FileSystemRights & WriteMask) == 0) continue;
                string sid = r.IdentityReference.Value;
                if (!TrustedSids.Contains(sid) && sid != "S-1-3-0") return false; // CREATOR OWNER resolves to the (trusted) owner
            }
            return true;
        }
    }
}
