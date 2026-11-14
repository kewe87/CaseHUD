// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.AccessControl;
using CaseHUD.Sensors.Cooling;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class CoolingAllowTests
    {
        private const string Key = "/lpc/nct6701d/0/control/6";
        private const string Other = "/lpc/nct6701d/0/control/1";
        private const string Ok = "{\"ok\":true,\"reads\":[]}";
        private const string Denied = "{\"ok\":false,\"error\":\"denied\"}";

        private sealed class Ports : IControlPorts
        {
            public readonly List<string> Calls = new List<string>();
            public bool Set(string key, int percent) { Calls.Add("set " + key + " " + percent); return true; }
            public bool Release(string key) { Calls.Add("release " + key); return true; }
            public int Releases => Calls.Count(c => c.StartsWith("release "));
        }

        private static string Hold(string key, int percent) =>
            "{\"v\":1,\"type\":\"hold\",\"key\":\"" + key + "\",\"percent\":" + percent + "}";

        private static string WithTempFile(string content, Action<string> body)
        {
            string dir = Path.Combine(Path.GetTempPath(), "casehud-test-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(dir);
            try
            {
                string p = Path.Combine(dir, "sensors.json");
                if (content != null) File.WriteAllText(p, content);
                body(p);
            }
            finally
            {
                Directory.Delete(dir, true);
            }
            return null;
        }

        private static readonly Func<FileSecurity, bool> Yes = _ => true;
        private static readonly Func<DirectorySecurity, bool> DirYes = _ => true;

        [Fact]
        public void AHeaderThatIsNotListedIsDenied()
        {
            var ports = new Ports();
            var list = new CoolingAllowList(() => new HashSet<string> { Key }, x => { }, () => 0);
            var s = new ScriptedStream(Frames_(Hold(Other, 60), Hold(Key, 60)));
            new CoolingConnection(new ControlHolds(ports, x => { }), new DenyLog(x => { }), list.IsAllowed).Serve(s);
            Assert.Equal(new[] { Denied, Ok }, s.Responses().ToList());
            Assert.Equal(new[] { "set " + Key + " 60", "release " + Key }, ports.Calls);
        }

        [Fact]
        public void NothingIsReleasedByDefault()
        {
            var ports = new Ports();
            var list = new CoolingAllowList(() => new HashSet<string>(), x => { }, () => 0);
            var s = new ScriptedStream(Frames_(Hold(Key, 60)));
            new CoolingConnection(new ControlHolds(ports, x => { }), new DenyLog(x => { }), list.IsAllowed).Serve(s);
            Assert.Equal(Denied, s.Responses().Single());
            Assert.Empty(ports.Calls);
        }

        [Fact]
        public void FloorAndFormatStillApplyToAReleasedHeader()
        {
            var ports = new Ports();
            var s = new ScriptedStream(Frames_(Hold(Key, 10)));
            new CoolingConnection(new ControlHolds(ports, x => { }), new DenyLog(x => { }), _ => true).Serve(s);
            Assert.Equal(Denied, s.Responses().Single());
            Assert.Empty(ports.Calls);
        }

        [Fact]
        public void TheFileIsReadAgainAfterTheCacheRanOut()
        {
            long now = 0;
            int reads = 0;
            var current = new HashSet<string> { Key };
            var list = new CoolingAllowList(() => { reads++; return new HashSet<string>(current); }, x => { }, () => now);
            Assert.True(list.IsAllowed(Key));
            current.Clear();
            now = CoolingAllowList.CacheMs - 1;
            Assert.True(list.IsAllowed(Key));
            Assert.Equal(1, reads);
            now = CoolingAllowList.CacheMs;
            Assert.False(list.IsAllowed(Key));
            Assert.Equal(2, reads);
        }

        [Fact]
        public void AFailingReadReleasesNothing()
        {
            var list = new CoolingAllowList(() => throw new IOException("gone"), x => { }, () => 0);
            Assert.False(list.IsAllowed(Key));
        }

        [Fact]
        public void ChangesAreLoggedOnceNotOnEveryRead()
        {
            long now = 0;
            var lines = new List<string>();
            var list = new CoolingAllowList(() => new HashSet<string> { Key }, lines.Add, () => now);
            for (int i = 0; i < 5; i++) { list.IsAllowed(Key); now += CoolingAllowList.CacheMs; }
            Assert.Single(lines);
            Assert.Contains(Key, lines[0]);
        }

        [Fact]
        public void AWithdrawnHeaderGoesBackWithinTheSilenceLimitEvenWhileTheClientLives()
        {
            long now = 0;
            bool listed = true;
            int releasesWhenAsked = -1;
            var ports = new Ports();
            var holds = new ControlHolds(ports, x => { }, () => now);
            int call = 0;
            Func<string, bool> allowed = k =>
            {
                call++;
                if (call == 2) { listed = false; now = CoolingConnection.SilentMs - 1; }
                if (call == 3) { now = CoolingConnection.SilentMs; holds.Sweep(); releasesWhenAsked = ports.Releases; }
                return listed;
            };
            var s = new ScriptedStream(Frames_(Hold(Key, 60), Hold(Key, 60), Hold(Key, 60)));
            new CoolingConnection(holds, new DenyLog(x => { }), allowed, new RateLimiter(1000)).Serve(s);
            Assert.Equal(new[] { Ok, Denied, Denied }, s.Responses().ToList());
            // The denied repeats did not renew the hold, so the sweep gave it back while the connection was still open.
            Assert.Equal(1, releasesWhenAsked);
            Assert.Equal(1, ports.Releases);
        }

        [Fact]
        public void ReadsTheListedHeaders()
        {
            WithTempFile("{\"smbusBridge\":true,\"coolingHeaders\":[\"" + Key + "\",\"/gpu-nvidia/0/control/0\",5,\"" + Other + "\"]}", p =>
            {
                var log = new List<string>();
                var keys = BridgeConfig.ReadCoolingHeaders(p, log.Add, Yes, DirYes);
                Assert.Equal(new[] { Other, Key }, keys.OrderBy(k => k).ToArray());
                Assert.Equal(2, log.Count(l => l.Contains("not a fan header")));
                Assert.True(BridgeConfig.ReadSmbusBridge(p, log.Add, Yes, DirYes));
            });
        }

        [Theory]
        [InlineData(null)]
        [InlineData("{}")]
        [InlineData("{\"coolingHeaders\":\"" + Key + "\"}")]
        [InlineData("{\"coolingHeaders\":null}")]
        [InlineData("{\"coolingHeaders\":[]}")]
        [InlineData("{broken")]
        public void MissingOrUnusableMeansNothingIsReleased(string content)
        {
            WithTempFile(content, p => Assert.Empty(BridgeConfig.ReadCoolingHeaders(p, x => { }, Yes, DirYes)));
        }

        [Fact]
        public void AFileThatIsNotTrustedReleasesNothingAndSaysCooling()
        {
            WithTempFile("{\"coolingHeaders\":[\"" + Key + "\"]}", p =>
            {
                var log = new List<string>();
                Assert.Empty(BridgeConfig.ReadCoolingHeaders(p, log.Add, _ => false, DirYes));
                Assert.Empty(BridgeConfig.ReadCoolingHeaders(p, log.Add, Yes, _ => false));
                Assert.Equal(2, log.Count);
                Assert.All(log, l => { Assert.StartsWith("cooling pipe:", l); Assert.Contains("ignoring", l); Assert.Contains("must be owned by an administrator", l); });
            });
        }

        [Fact]
        public void AMissingDirectoryReleasesNothing()
        {
            string p = Path.Combine(Path.GetTempPath(), "casehud-test-" + Guid.NewGuid().ToString("N"), "sensors.json");
            Assert.Empty(BridgeConfig.ReadCoolingHeaders(p, x => { }, Yes, DirYes));
        }


        // What a new file in the hardened folder looks like: owner Administrators plus the inherited
        // entries the installer sets on the folder (SYSTEM and Administrators full, Users read and execute).
        private static FileSecurity SecurityOfNewFile(string owner, string user)
        {
            var sec = new FileSecurity();
            sec.SetAccessRuleProtection(true, false);
            sec.SetOwner(new System.Security.Principal.SecurityIdentifier(owner));
            sec.AddAccessRule(new FileSystemAccessRule(new System.Security.Principal.SecurityIdentifier("S-1-5-18"), FileSystemRights.FullControl, AccessControlType.Allow));
            sec.AddAccessRule(new FileSystemAccessRule(new System.Security.Principal.SecurityIdentifier("S-1-5-32-544"), FileSystemRights.FullControl, AccessControlType.Allow));
            sec.AddAccessRule(new FileSystemAccessRule(new System.Security.Principal.SecurityIdentifier(user), FileSystemRights.ReadAndExecute | FileSystemRights.Synchronize, AccessControlType.Allow));
            return sec;
        }

        [Fact]
        public void AFileInTheHardenedFolderOwnedByAdministratorsIsTrusted()
        {
            Assert.True(BridgeConfig.IsTrusted(SecurityOfNewFile("S-1-5-32-544", "S-1-5-32-545")));
            Assert.False(BridgeConfig.IsTrusted(SecurityOfNewFile("S-1-5-21-1-2-3-1001", "S-1-5-32-545")), "owned by a normal user");
        }
        [Fact]
        public void TheDefaultListGoesThroughTheSameTrustCheck()
        {
            // The real file is not touched: with no trust override a temp file owned by this user is refused.
            WithTempFile("{\"coolingHeaders\":[\"" + Key + "\"]}", p =>
            {
                var log = new List<string>();
                Assert.Empty(BridgeConfig.ReadCoolingHeaders(p, log.Add));
                Assert.Contains(log, l => l.Contains("ignoring"));
            });
        }
    }
}
