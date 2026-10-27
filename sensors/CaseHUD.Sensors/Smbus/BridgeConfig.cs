// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Web.Script.Serialization;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Reads the service settings (C:\ProgramData\CaseHUD\sensors.json). The only key
    /// today is "smbusBridge" (default false). ProgramData\CaseHUD is locked down by the
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

        /// <summary>
        /// True only if the file exists, is trusted and contains <c>"smbusBridge": true</c>.
        /// The directory must be an ordinary directory (no junction) that only administrators
        /// can modify or take over; the file must not be a symbolic link either.
        /// </summary>
        public static bool ReadSmbusBridge(string path, Action<string> log, Func<FileSecurity, bool> trusted = null, Func<DirectorySecurity, bool> dirTrusted = null)
        {
            try
            {
                string dir = System.IO.Path.GetDirectoryName(path);
                var di = new DirectoryInfo(dir);
                if (!di.Exists) return false;
                if ((di.Attributes & FileAttributes.ReparsePoint) != 0)
                {
                    log("SMBus bridge: ignoring " + path + " (" + dir + " is a reparse point)");
                    return false;
                }
                if (!File.Exists(path)) return false;
                if ((new FileInfo(path).Attributes & FileAttributes.ReparsePoint) != 0)
                {
                    log("SMBus bridge: ignoring " + path + " (the file is a reparse point)");
                    return false;
                }
                if (!(dirTrusted ?? IsDirectoryTrusted)(di.GetAccessControl()))
                {
                    log("SMBus bridge: ignoring " + path + " (" + dir + " must be owned by an administrator and not allow other users to delete or replace files in it)");
                    return false;
                }
                using (var fs = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read))
                {
                    if (!(trusted ?? IsTrusted)(fs.GetAccessControl()))
                    {
                        log("SMBus bridge: ignoring " + path + " (must be owned by an administrator and not writable by other users)");
                        return false;
                    }
                    if (fs.Length > 64 * 1024) return false;
                    string json = new StreamReader(fs).ReadToEnd();
                    var obj = new JavaScriptSerializer().DeserializeObject(json) as Dictionary<string, object>;
                    object v;
                    return obj != null && obj.TryGetValue("smbusBridge", out v) && v is bool && (bool)v;
                }
            }
            catch (FileNotFoundException) { return false; }
            catch (DirectoryNotFoundException) { return false; }
            catch (Exception e)
            {
                log("SMBus bridge: reading " + path + " failed: " + e.Message);
                return false;
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
