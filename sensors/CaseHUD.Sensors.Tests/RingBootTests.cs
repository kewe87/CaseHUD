// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using CaseHUD.Sensors.RingBoot;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class RingBootTests
    {
        private static RingBootState Parse(string json) => RingBootState.ParseFile(Encoding.UTF8.GetBytes(json));

        [Fact]
        public void ParsesValidStates()
        {
            Assert.Equal(RingBootState.Off, Parse("{\"mode\":\"off\",\"r\":0,\"g\":0,\"b\":0}"));
            Assert.Equal(RingBootState.Off, Parse("{\"mode\":\"off\"}"));
            Assert.Equal(RingBootState.None, Parse("{\"mode\":\"none\",\"r\":9,\"g\":9,\"b\":9}"));
            RingBootState c = Parse("{\"mode\":\"color\",\"r\":255,\"g\":0,\"b\":17}");
            Assert.Equal(RingBootMode.Color, c.Mode);
            Assert.Equal(new byte[] { 255, 0, 17 }, new[] { c.R, c.G, c.B });
        }

        [Theory]
        [InlineData("")]
        [InlineData("not json")]
        [InlineData("[]")]
        [InlineData("\"off\"")]
        [InlineData("{}")]
        [InlineData("{\"mode\":\"OFF\"}")]
        [InlineData("{\"mode\":\"rainbow\"}")]
        [InlineData("{\"mode\":1}")]
        [InlineData("{\"mode\":null}")]
        [InlineData("{\"mode\":\"color\"}")]
        [InlineData("{\"mode\":\"color\",\"r\":1,\"g\":2}")]
        [InlineData("{\"mode\":\"color\",\"r\":256,\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":-1,\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":1.5,\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":\"1\",\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":true,\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":null,\"g\":0,\"b\":0}")]
        [InlineData("{\"mode\":\"color\",\"r\":1,\"g\":2,\"b\":3,\"path\":\"C:\\\\x\"}")]
        [InlineData("{\"mode\":\"off\",\"r\":999}")]
        public void InvalidMeansNone(string json)
        {
            Assert.Equal(RingBootState.None, Parse(json));
        }

        [Fact]
        public void OversizedFileMeansNone()
        {
            string pad = new string(' ', RingBootState.MaxJsonBytes);
            Assert.Equal(RingBootState.None, Parse("{\"mode\":\"off\"}" + pad));
            Assert.Equal(RingBootState.None, RingBootState.ParseFile(null));
            Assert.Equal(RingBootState.None, RingBootState.ParseFile(new byte[] { 0xFF, 0xFE, 0x7B })); // invalid UTF-8
        }

        [Fact]
        public void JsonRoundTrips()
        {
            foreach (RingBootState s in new[] { RingBootState.None, RingBootState.Off, RingBootState.FromColor(1, 2, 3) })
                Assert.Equal(s, Parse(s.ToJson()));
            Assert.Equal("{\"mode\":\"color\",\"r\":1,\"g\":2,\"b\":3}", RingBootState.FromColor(1, 2, 3).ToJson());
        }

        // Layout of the Go driver (internal/rgb/backend/lianli): per packet byte 0 = 0x11,
        // byte 1 = first LED (0, 20, 40), bytes 2..3 = 0, then 20 x R G B.
        [Fact]
        public void FrameIsByteExact()
        {
            byte[][] f = RingFrame.Build(0x11, 0x22, 0x33);
            Assert.Equal(3, f.Length);
            for (int p = 0; p < 3; p++)
            {
                Assert.Equal(64, f[p].Length);
                Assert.Equal(0x11, f[p][0]);
                Assert.Equal(p * 20, f[p][1]);
                Assert.Equal(0, f[p][2]);
                Assert.Equal(0, f[p][3]);
                for (int i = 0; i < 20; i++)
                {
                    Assert.Equal(0x11, f[p][4 + 3 * i]);
                    Assert.Equal(0x22, f[p][5 + 3 * i]);
                    Assert.Equal(0x33, f[p][6 + 3 * i]);
                }
            }
            // whole first packet spelled out
            var expect = new byte[64];
            expect[0] = 0x11;
            for (int i = 0; i < 20; i++) { expect[4 + 3 * i] = 0x11; expect[5 + 3 * i] = 0x22; expect[6 + 3 * i] = 0x33; }
            Assert.Equal(expect, f[0]);
        }

        [Fact]
        public void OffAndNoneBuildBlack()
        {
            foreach (byte[] pkt in RingFrame.Build(RingBootState.Off)) Assert.All(pkt.Skip(2), b => Assert.Equal(0, b));
            Assert.Equal(RingFrame.Build(5, 6, 7), RingFrame.Build(RingBootState.FromColor(5, 6, 7)));
        }

        [Fact]
        public void PacketGuardAllowsOnly0x11()
        {
            Assert.True(RingFrame.IsValidPacket(RingFrame.Build(1, 2, 3)[2]));
            foreach (byte cmd in new byte[] { 0x10, 0x13, 0x20, 0x21, 0x22, 0x23, 0x00 })
            {
                byte[] p = RingFrame.Build(1, 2, 3)[0];
                p[0] = cmd;
                Assert.False(RingFrame.IsValidPacket(p));
            }
            byte[] odd = RingFrame.Build(1, 2, 3)[0];
            odd[1] = 7;
            Assert.False(RingFrame.IsValidPacket(odd));
            Assert.False(RingFrame.IsValidPacket(new byte[63]));
            Assert.False(RingFrame.IsValidPacket(null));
        }

        // ---- applier ----

        private sealed class FakeDevice : IRingDevice
        {
            private readonly FakeUsb _usb;
            public FakeDevice(FakeUsb usb) { _usb = usb; _usb.OpenHandles++; }
            public void Write(byte[] packet)
            {
                if (_usb.FailWrite) throw new IOException("write failed");
                _usb.Written.Add(packet);
            }
            public void Dispose() { _usb.OpenHandles--; _usb.Disposed++; }
        }

        private sealed class FakeUsb : IRingUsb
        {
            public readonly List<byte[]> Written = new List<byte[]>();
            public int OpenHandles, Disposed, Finds, OpenFailures;
            public int RingAppearsAtFind = 1;
            public bool FailWrite;
            public IList<string> FindRings() => ++Finds >= RingAppearsAtFind ? new List<string> { "ring" } : new List<string>();
            public IRingDevice Open(string path)
            {
                if (OpenFailures > 0) { OpenFailures--; throw new IOException("busy"); }
                return new FakeDevice(this);
            }
        }

        private sealed class Clock
        {
            public long Now;
            public int Sleeps;
            public Action<int> Sleep => ms => { Now += ms; Sleeps++; };
        }

        private static bool Apply(FakeUsb usb, RingBootState s, List<string> log, Clock clock = null, CancellationToken ct = default(CancellationToken), long uptimeMs = 0)
        {
            clock = clock ?? new Clock();
            return new RingBootApplier(usb, log.Add, () => clock.Now, clock.Sleep, () => uptimeMs).Apply(s, ct);
        }

        [Fact]
        public void NoneNeverTouchesUsb()
        {
            var usb = new FakeUsb();
            var log = new List<string>();
            Assert.False(Apply(usb, RingBootState.None, log));
            Assert.Equal(0, usb.Finds);
            Assert.Single(log);
        }

        [Fact]
        public void WritesOneFrameAndReleasesHandle()
        {
            var usb = new FakeUsb();
            var log = new List<string>();
            Assert.True(Apply(usb, RingBootState.FromColor(9, 8, 7), log));
            Assert.Equal(3, usb.Written.Count);
            Assert.Equal(RingFrame.Build(9, 8, 7), usb.Written);
            Assert.All(usb.Written, p => Assert.Equal(0x11, p[0]));
            Assert.Equal(0, usb.OpenHandles);
            Assert.Equal(1, usb.Disposed);
            Assert.Single(log);
        }

        [Fact]
        public void OffWritesBlack()
        {
            var usb = new FakeUsb();
            Assert.True(Apply(usb, RingBootState.Off, new List<string>()));
            Assert.All(usb.Written, p => Assert.All(p.Skip(2), b => Assert.Equal(0, b)));
        }

        [Fact]
        public void RetriesUntilRingAppears()
        {
            var usb = new FakeUsb { RingAppearsAtFind = 4, OpenFailures = 1 };
            var log = new List<string>();
            var clock = new Clock();
            Assert.True(Apply(usb, RingBootState.Off, log, clock));
            Assert.True(clock.Sleeps >= 3);
            Assert.Equal(3, usb.Written.Count);
            Assert.Equal(0, usb.OpenHandles);
            Assert.Single(log);
        }

        [Fact]
        public void GivesUpAfterTheWindow()
        {
            var usb = new FakeUsb { RingAppearsAtFind = int.MaxValue };
            var log = new List<string>();
            var clock = new Clock();
            Assert.False(Apply(usb, RingBootState.Off, log, clock));
            Assert.True(clock.Now <= RingBootApplier.DefaultWindowMs);
            Assert.True(clock.Sleeps >= 10);
            Assert.Empty(usb.Written);
            Assert.Single(log);
            Assert.Contains("gave up", log[0]);
        }

        [Fact]
        public void WriteFailureStillReleasesHandle()
        {
            var usb = new FakeUsb { FailWrite = true };
            var log = new List<string>();
            Assert.False(Apply(usb, RingBootState.Off, log));
            Assert.Equal(0, usb.OpenHandles);
            Assert.True(usb.Disposed >= 1);
            Assert.Single(log);
        }

        [Fact]
        public void CancelStopsRetrying()
        {
            var usb = new FakeUsb { RingAppearsAtFind = int.MaxValue };
            var cts = new CancellationTokenSource();
            cts.Cancel();
            var clock = new Clock();
            Assert.False(Apply(usb, RingBootState.Off, new List<string>(), clock, cts.Token));
            Assert.Equal(0, clock.Sleeps);
        }

        // ---- store ----

        private static string TempPath()
        {
            string dir = Path.Combine(Path.GetTempPath(), "casehud-ringboot-" + Guid.NewGuid().ToString("N"));
            return Path.Combine(dir, "ring-boot.json");
        }

        // The directory is created up front: EnsureDirectory would give it the service ACL
        // (SYSTEM/Administrators only), which a test running as a normal user cannot write to.
        private static RingBootStore NewStore(string path, List<string> log = null, bool trusted = true)
        {
            Directory.CreateDirectory(Path.GetDirectoryName(path));
            return new RingBootStore(path, (log ?? new List<string>()).Add, f => trusted, d => trusted);
        }

        [Fact]
        public void StoreSavesAtomicallyAndLoads()
        {
            string path = TempPath();
            try
            {
                var store = NewStore(path);
                Assert.Equal(RingBootState.None, store.Load()); // no file yet
                Assert.True(store.Save(RingBootState.FromColor(1, 2, 3)));
                Assert.Equal(RingBootState.FromColor(1, 2, 3), store.Load());
                Assert.True(store.Save(RingBootState.Off));
                Assert.Equal(RingBootState.Off, store.Load());
                Assert.False(File.Exists(path + ".tmp"));
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void UntrustedFileMeansNone()
        {
            string path = TempPath();
            try
            {
                Assert.True(NewStore(path).Save(RingBootState.Off));
                var log = new List<string>();
                Assert.Equal(RingBootState.None, NewStore(path, log, trusted: false).Load());
                Assert.Single(log);
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void CorruptFileMeansNone()
        {
            string path = TempPath();
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, "{\"mode\":\"color\",\"r\":300,\"g\":0,\"b\":0}");
                Assert.Equal(RingBootState.None, NewStore(path).Load());
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void DirectoryAclIsAdminOnlyWrite()
        {
            var sec = RingBootStore.CreateSecurity();
            Assert.True(sec.AreAccessRulesProtected);
            Assert.True(BridgeConfig.IsDirectoryTrusted(WithOwner(sec)));
        }

        private static System.Security.AccessControl.DirectorySecurity WithOwner(System.Security.AccessControl.DirectorySecurity s)
        {
            s.SetOwner(new System.Security.Principal.SecurityIdentifier(System.Security.Principal.WellKnownSidType.BuiltinAdministratorsSid, null));
            return s;
        }

        // ---- pipe request ----

        private const string Denied = "{\"ok\":false,\"error\":\"denied\"}";
        private const string Ok = "{\"ok\":true,\"reads\":[]}";
        private const string Io = "{\"ok\":false,\"error\":\"io\"}";

        private static string Send(string json, Func<RingBootState, bool> handler, out List<string> denyLog)
        {
            // the ring pipe's connection class is the only one that accepts setRingBoot
            var log = new List<string>();
            var conn = new RingConnection(handler, new DenyLog(log.Add));
            var s = new ScriptedStream(Frame(json));
            conn.Serve(s);
            denyLog = log;
            return s.Responses().Single();
        }

        private static string Send(string json, Func<RingBootState, bool> handler)
        {
            List<string> l;
            return Send(json, handler, out l);
        }

        [Fact]
        public void ValidRequestReachesTheHandler()
        {
            RingBootState got = null;
            Assert.Equal(Ok, Send("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":10,\"g\":20,\"b\":30}", s => { got = s; return true; }));
            Assert.Equal(RingBootState.FromColor(10, 20, 30), got);

            Assert.Equal(Ok, Send("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}", s => { got = s; return true; }));
            Assert.Equal(RingBootState.Off, got);

            Assert.Equal(Ok, Send("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"none\",\"r\":0,\"g\":0,\"b\":0}", s => { got = s; return true; }));
            Assert.Equal(RingBootState.None, got);
        }

        [Fact]
        public void HandlerFailureIsIo()
        {
            Assert.Equal(Io, Send("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}", s => false));
        }

        [Theory]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\"}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"rainbow\"}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":1,\"g\":2}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":256,\"g\":0,\"b\":0}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":\"1\",\"g\":0,\"b\":0}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":1.5,\"g\":0,\"b\":0}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"color\",\"r\":true,\"g\":0,\"b\":0}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":true}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\",\"path\":\"C:\\\\x\"}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\",\"addr\":112}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\",\"ops\":[]}")]
        [InlineData("{\"v\":2,\"type\":\"setRingBoot\",\"mode\":\"off\"}")]
        [InlineData("{\"v\":\"1\",\"type\":\"setRingBoot\",\"mode\":\"off\"}")]
        [InlineData("{\"type\":\"setRingBoot\",\"mode\":\"off\"}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoots\",\"mode\":\"off\"}")]
        [InlineData("{\"v\":1,\"type\":5,\"mode\":\"off\"}")]
        [InlineData("{\"v\":1,\"type\":null,\"mode\":\"off\"}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":[\"off\"]}")]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":{\"x\":1}}")]
        public void InvalidRequestIsDenied(string json)
        {
            bool called = false;
            List<string> log;
            Assert.Equal(Denied, Send(json, s => { called = true; return true; }, out log));
            Assert.False(called);
        }

        [Fact]
        public void OversizedRequestIsDenied()
        {
            bool called = false;
            string json = "{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\",\"pad\":\"" + new string('a', RingBootProtocol.MaxBody) + "\"}";
            Assert.Equal(Denied, Send(json, s => { called = true; return true; }));
            Assert.False(called);
            // a message above the global limit drops the connection without any answer
            var conn = new RingConnection(s => true, new DenyLog(x => { }));
            var big = new byte[4];
            BitConverter.GetBytes((uint)(BridgeProtocol.MaxMessage + 1)).CopyTo(big, 0);
            var s2 = new ScriptedStream(big);
            conn.Serve(s2);
            Assert.Empty(s2.Responses());
        }

        [Fact]
        public void SmbusPipeDeniesSetRingBoot()
        {
            var bus = new FakeBus();
            var lk = new FakeLock();
            var exec = new BridgeExecutor(bus, lk, x => { });
            for (byte a = 0x70; a <= 0x77; a++) exec.Verifier.Mark(a);
            var conn = new BridgeConnection(exec, new DenyLog(x => { }));
            var s = new ScriptedStream(Frame("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}"));
            conn.Serve(s);
            Assert.Equal(Denied, s.Responses().Single());
            Assert.Empty(bus.Calls);
            Assert.Equal(0, lk.Acquired);
        }

        [Fact]
        public void RingPipeDeniesSmbusRequests()
        {
            bool called = false;
            Assert.Equal(Denied, Send(Req("{\"op\":\"rcmd\",\"cmd\":160}"), s => { called = true; return true; }));
            Assert.Equal(Denied, Send("{\"v\":1,\"type\":\"smbus\",\"addr\":113,\"lockMs\":10,\"ops\":[]}", s => { called = true; return true; }));
            Assert.Equal(Denied, Send("not json", s => { called = true; return true; }));
            Assert.False(called);
        }

        [Fact]
        public void RingPipeRateLimits()
        {
            long now = 0;
            var conn = new RingConnection(s => true, new DenyLog(x => { }), new RateLimiter(1, () => now));
            var st = new ScriptedStream(Frames_(
                "{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}",
                "{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}"));
            conn.Serve(st);
            Assert.Equal(new[] { Ok, "{\"ok\":false,\"error\":\"timeout\"}" }, st.Responses().ToArray());
        }

        [Fact]
        public void RingPipeUsesTheSharedServerClass()
        {
            // same server class as the SMBus pipe: ACL, remote rejection, caps and idle sweep come with it
            Assert.Equal("CaseHUD.Ring", RingConnection.PipeName);
            Assert.NotEqual(SmbusBridgeHost.PipeName, RingConnection.PipeName);
        }

        [Fact]
        public void SmbusRequestsStillWork()
        {
            var bus = new FakeBus();
            var exec = new BridgeExecutor(bus, new FakeLock(), x => { });
            for (byte a = 0x70; a <= 0x77; a++) exec.Verifier.Mark(a);
            var conn = new BridgeConnection(exec, new DenyLog(x => { }));
            var st = new ScriptedStream(Frame(Req("{\"op\":\"rcmd\",\"cmd\":160}")));
            conn.Serve(st);
            Assert.Equal("{\"ok\":true,\"reads\":[90]}", st.Responses().Single());
        }

        [Fact]
        public void SkipsWhenNotAtBoot()
        {
            var usb = new FakeUsb();
            var log = new List<string>();
            Assert.False(Apply(usb, RingBootState.Off, log, uptimeMs: RingBootApplier.MaxUptimeMs + 1));
            Assert.Equal(0, usb.Finds);
            Assert.Empty(usb.Written);
            Assert.Single(log);
            Assert.Contains("not at boot", log[0]);
            Assert.True(Apply(new FakeUsb(), RingBootState.Off, new List<string>(), uptimeMs: RingBootApplier.MaxUptimeMs));
        }

        [Fact]
        public void RealUptimeIsPositive() => Assert.True(SystemUptime.Ms() > 0);

        private sealed class HandlerRig : IDisposable
        {
            public readonly string Path = TempPath();
            public readonly List<string> Log = new List<string>();
            public long Now = 100000;
            public readonly List<KeyValuePair<int, Action>> Timers = new List<KeyValuePair<int, Action>>();
            public readonly RingBootRequestHandler H;
            public readonly RingBootStore Store;

            public HandlerRig(bool trusted = true)
            {
                Store = NewStore(Path, Log, trusted);
                H = new RingBootRequestHandler(Store, Log.Add, () => Now, (ms, a) => Timers.Add(new KeyValuePair<int, Action>(ms, a)));
            }

            public int Sets => Log.Count(l => l.Contains("set to"));
            public void Dispose() { try { Directory.Delete(System.IO.Path.GetDirectoryName(Path), true); } catch { } }
        }

        [Fact]
        public void HandlerCoalescesWritesWithTrailingWrite()
        {
            using (var r = new HandlerRig())
            {
                Assert.True(r.H.Handle(RingBootState.Off));
                Assert.True(r.H.Handle(RingBootState.Off));   // unchanged
                Assert.Equal(1, r.Sets);
                // inside the window: nothing written, one timer for the end of the window
                r.Now += 1000;
                Assert.True(r.H.Handle(RingBootState.FromColor(1, 1, 1)));
                r.Now += 1000;
                Assert.True(r.H.Handle(RingBootState.FromColor(2, 2, 2)));
                Assert.Equal(RingBootState.Off, r.Store.Load());
                Assert.Single(r.Timers);
                Assert.Equal(1, r.Sets);
                // the window ends: the latest request is written
                r.Now += 3000;
                r.Timers[0].Value();
                Assert.Equal(RingBootState.FromColor(2, 2, 2), r.Store.Load());
                Assert.Equal(2, r.Sets);
                // after the window a request is written at once
                r.Now += 5000;
                Assert.True(r.H.Handle(RingBootState.FromColor(3, 3, 3)));
                Assert.Equal(RingBootState.FromColor(3, 3, 3), r.Store.Load());
                Assert.Equal(3, r.Sets);
            }
        }

        [Fact]
        public void PendingRequestBackToSavedStateWritesNothing()
        {
            using (var r = new HandlerRig())
            {
                r.H.Handle(RingBootState.Off);
                r.Now += 1000;
                r.H.Handle(RingBootState.FromColor(1, 1, 1));
                r.H.Handle(RingBootState.Off); // back to what is on disk
                r.Now += 4000;
                r.Timers[0].Value();
                Assert.Equal(1, r.Sets);
                Assert.Equal(RingBootState.Off, r.Store.Load());
            }
        }

        [Fact]
        public void UnreadableFileIsNotSeeded()
        {
            string path = TempPath();
            try
            {
                var log = new List<string>();
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, "garbage");
                var store = NewStore(path, log);
                RingBootState s;
                Assert.Equal(RingBootStore.LoadStatus.Unreadable, store.TryLoad(out s));
                var h = new RingBootRequestHandler(store, log.Add, () => 0, (ms, a) => { });
                Assert.True(h.Handle(RingBootState.None)); // equals "none" but must still be written
                Assert.Equal(RingBootStore.LoadStatus.Valid, store.TryLoad(out s));
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void LoadIsTriState()
        {
            string path = TempPath();
            try
            {
                var store = NewStore(path);
                RingBootState s;
                Assert.Equal(RingBootStore.LoadStatus.Absent, store.TryLoad(out s));
                store.Save(RingBootState.FromColor(4, 5, 6));
                Assert.Equal(RingBootStore.LoadStatus.Valid, store.TryLoad(out s));
                Assert.Equal(RingBootState.FromColor(4, 5, 6), s);
                Assert.Equal(RingBootStore.LoadStatus.Unreadable, NewStore(path, null, trusted: false).TryLoad(out s));
                Assert.Equal(RingBootState.None, s);
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void SaveRefusesUntrustedDirectory()
        {
            using (var r = new HandlerRig(trusted: false))
            {
                Assert.False(r.Store.Save(RingBootState.Off));
                Assert.False(r.H.Handle(RingBootState.Off));
                Assert.False(File.Exists(r.Path));
                Assert.Empty(Directory.GetFiles(System.IO.Path.GetDirectoryName(r.Path))); // no temp file left behind
            }
        }

        [Fact]
        public void SaveUsesRandomTempAndCleansUp()
        {
            string path = TempPath();
            try
            {
                var store = NewStore(path);
                // a predictable temp name left by someone else is never touched or reused
                string decoy = path + ".tmp";
                File.WriteAllText(decoy, "decoy");
                Assert.True(store.Save(RingBootState.Off));
                Assert.Equal("decoy", File.ReadAllText(decoy));
                Assert.Equal(new[] { decoy, path }.OrderBy(x => x), Directory.GetFiles(Path.GetDirectoryName(path)).OrderBy(x => x));
                // failure (target is a directory) removes the random temp file
                File.Delete(path);
                Directory.CreateDirectory(path);
                Assert.False(store.Save(RingBootState.FromColor(1, 2, 3)));
                Assert.Equal(new[] { decoy }, Directory.GetFiles(Path.GetDirectoryName(path)));
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void SharedLimiterIsChargedOnlyForValidRequests()
        {
            var shared = new RateLimiter(1, () => 0);
            var conn = new RingConnection(s => true, new DenyLog(x => { }), new RateLimiter(100, () => 0), shared);
            string bad = "{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"nope\"}";
            string good = "{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}";
            var st = new ScriptedStream(Frames_(bad, bad, bad, good, good));
            conn.Serve(st);
            string timeout = "{\"ok\":false,\"error\":\"timeout\"}";
            Assert.Equal(new[] { Denied, Denied, Denied, Ok, timeout }, st.Responses().ToArray());
        }
    }
}
