// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Text;
using System.Threading;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class HardeningTests
    {
        private static readonly string Query = Req("{\"op\":\"rcmd\",\"cmd\":161}");

        private static bool RoundTrip(Stream c)
        {
            byte[] f = Frame(Query);
            c.Write(f, 0, f.Length);
            byte[] body;
            return BridgeProtocol.ReadMessage(c, out body) == BridgeProtocol.ReadResult.Ok
                && Encoding.UTF8.GetString(body) == "{\"ok\":true,\"reads\":[90]}";
        }

        private static NamedPipeClientStream Connect(string name)
        {
            var c = new NamedPipeClientStream(".", name, PipeDirection.InOut);
            c.Connect(3000);
            return c;
        }

        private static string NewName() => "CaseHUD.Smbus.Test." + Guid.NewGuid().ToString("N");

        private static NamedPipeBridgeServer NewServer(string name, int max = 8, int perProcess = 8, long idleMs = 60000, int sweepMs = 5000)
        {
            var exec = new BridgeExecutor(new FakeBus(), new FakeLock(), s => { });
            var deny = new DenyLog(s => { });
            return new NamedPipeBridgeServer(name, s => new BridgeConnection(exec, deny).Serve(s), s => { }, max, perProcess, idleMs, sweepMs);
        }

        private static BridgeProtocol.ReadResult ReadOrClosed(Stream s, out byte[] body)
        {
            body = null;
            try { return BridgeProtocol.ReadMessage(s, out body); }
            catch (IOException) { return BridgeProtocol.ReadResult.Closed; }
        }

        // the accept loop survives the connection cap
        [Fact]
        public void ServerKeepsServingAfterConnectionCapIsHit()
        {
            string name = NewName();
            using (var srv = NewServer(name, max: 8, perProcess: 8))
            {
                srv.Start();
                var held = new List<NamedPipeClientStream>();
                for (int i = 0; i < 8; i++)
                {
                    var c = Connect(name);
                    Assert.True(RoundTrip(c));
                    held.Add(c);
                }
                // the 9th is accepted at the pipe level and then dropped by the server
                var ninth = Connect(name);
                byte[] f = Frame(Query);
                byte[] body = null;
                try
                {
                    ninth.Write(f, 0, f.Length);
                    BridgeProtocol.ReadMessage(ninth, out body);
                }
                catch (IOException) { }
                Assert.Null(body);
                ninth.Dispose();
                Assert.True(srv.IsAlive);

                held[0].Dispose();
                held.RemoveAt(0);
                bool ok = false;
                for (int i = 0; i < 40 && !ok; i++)
                {
                    try
                    {
                        using (var c = Connect(name)) ok = RoundTrip(c);
                    }
                    catch (IOException) { Thread.Sleep(50); }
                }
                Assert.True(ok);
                Assert.True(srv.IsAlive);
                foreach (var c in held) c.Dispose();
            }
        }

        [Fact]
        public void SecondServerOnSameNameFailsToStart()
        {
            string name = NewName();
            using (var a = NewServer(name))
            {
                a.Start();
                using (var b = NewServer(name))
                {
                    Assert.ThrowsAny<IOException>(() => b.Start());
                    Assert.False(b.IsAlive);
                }
                using (var c = Connect(name)) Assert.True(RoundTrip(c));
            }
            using (var d = NewServer(name))
            {
                d.Start(); // free again after dispose
                using (var c = Connect(name)) Assert.True(RoundTrip(c));
            }
        }

        [Fact]
        public void HostStaysOffWhenNameIsTakenAndRetries()
        {
            string name = NewName();
            var log = new List<string>();
            var squatter = NewServer(name);
            squatter.Start();
            var host = new SmbusBridgeHost(() => true, () => NewServer(name), log.Add);
            host.Poll();
            host.Poll();
            Assert.False(host.IsRunning);
            Assert.Single(log);
            squatter.Dispose();
            host.Poll();
            Assert.True(host.IsRunning);
            host.Dispose();
        }

        private sealed class DeadablePipe : IPipeServer
        {
            public bool Alive = true, Disposed;
            public bool IsAlive => Alive;
            public void Start() { }
            public void Dispose() => Disposed = true;
        }

        [Fact]
        public void HostRestartsDeadServer()
        {
            var pipes = new List<DeadablePipe>();
            var host = new SmbusBridgeHost(() => true, () => { var p = new DeadablePipe(); pipes.Add(p); return p; }, s => { });
            host.Poll();
            pipes[0].Alive = false;
            host.Poll();
            Assert.Equal(2, pipes.Count);
            Assert.True(pipes[0].Disposed);
            Assert.True(host.IsRunning);
        }

        [Fact]
        public void IdleConnectionIsClosed()
        {
            string name = NewName();
            using (var srv = NewServer(name, idleMs: 300, sweepMs: 100))
            {
                srv.Start();
                using (var c = Connect(name))
                {
                    Assert.True(RoundTrip(c));
                    byte[] body;
                    Assert.NotEqual(BridgeProtocol.ReadResult.Ok, ReadOrClosed(c, out body)); // server hangs up while idle
                }
                using (var c = Connect(name)) Assert.True(RoundTrip(c));
            }
        }

        [Fact]
        public void IdleSweepRunsDespiteConstantReconnects()
        {
            string name = NewName();
            using (var srv = NewServer(name, idleMs: 300, sweepMs: 100))
            {
                srv.Start();
                using (var idle = Connect(name))
                {
                    Assert.True(RoundTrip(idle));
                    var stop = new System.Threading.ManualResetEvent(false);
                    var churn = new System.Threading.Thread(() =>
                    {
                        while (!stop.WaitOne(20))
                        {
                            try { using (var c = Connect(name)) RoundTrip(c); } catch (Exception) { }
                        }
                    });
                    churn.Start();
                    try
                    {
                        byte[] body;
                        Assert.NotEqual(BridgeProtocol.ReadResult.Ok, ReadOrClosed(idle, out body));
                    }
                    finally
                    {
                        stop.Set();
                        churn.Join();
                    }
                }
            }
        }

        [Fact]
        public void TrackerLimitsTotalAndPerProcess()
        {
            var t = new ConnectionTracker(3, 2);
            Stream a = t.TryAdd(new MemoryStream(), 10);
            Stream b = t.TryAdd(new MemoryStream(), 10);
            Assert.NotNull(a);
            Assert.NotNull(b);
            Assert.Null(t.TryAdd(new MemoryStream(), 10)); // third from the same process
            Stream c = t.TryAdd(new MemoryStream(), 11);
            Assert.NotNull(c);
            Assert.Null(t.TryAdd(new MemoryStream(), 12)); // total reached
            a.Dispose();
            Assert.Equal(2, t.Count);
            Assert.NotNull(t.TryAdd(new MemoryStream(), 10));
        }

        private sealed class Tracked : MemoryStream
        {
            public bool Closed;
            protected override void Dispose(bool disposing)
            {
                Closed = true;
                base.Dispose(disposing);
            }
        }

        [Fact]
        public void TrackerClosesIdleOnly()
        {
            long now = 0;
            var t = new ConnectionTracker(8, 8, () => now);
            var rawA = new Tracked();
            var rawB = new Tracked();
            t.TryAdd(rawA, 1);
            Stream b = t.TryAdd(rawB, 2);
            now = 50000;
            b.Write(new byte[1], 0, 1); // B is active
            now = 61000;
            Assert.Equal(1, t.CloseIdle(60000));
            Assert.True(rawA.Closed);
            Assert.False(rawB.Closed);
        }

        [Fact]
        public void TrackerRefusesAfterCloseAll()
        {
            var t = new ConnectionTracker(8, 8);
            var raw = new Tracked();
            t.TryAdd(raw, 1);
            t.CloseAll();
            Assert.True(raw.Closed);
            Assert.Null(t.TryAdd(new MemoryStream(), 2)); // no connection slips in during shutdown
        }

        [Fact]
        public void SharedLimiterCapsAllConnections()
        {
            var shared = new RateLimiter(240, () => 0);
            var exec = new BridgeExecutor(new FakeBus(), new FakeLock(), s => { });
            int ok = 0, timeouts = 0;
            for (int c = 0; c < 4; c++) // 4 connections x 100 batches, each below its own limit
            {
                var conn = new BridgeConnection(exec, new DenyLog(s => { }), new RateLimiter(120, () => 0), shared);
                var input = new MemoryStream();
                for (int i = 0; i < 100; i++) { byte[] f = Frame(Query); input.Write(f, 0, f.Length); }
                var stream = new ScriptedStream(input.ToArray());
                conn.Serve(stream);
                foreach (string r in stream.Responses()) { if (r.StartsWith("{\"ok\":true")) ok++; else timeouts++; }
            }
            Assert.Equal(240, ok);
            Assert.Equal(160, timeouts);
        }

        [Fact]
        public void HoldBudgetLimitsMutexTime()
        {
            long now = 0;
            var budget = new HoldBudget(() => now);
            Assert.True(budget.Allow());
            budget.Record(499000);
            Assert.True(budget.Allow());
            budget.Record(2000);
            Assert.False(budget.Allow());
            now = 1000000;
            Assert.True(budget.Allow()); // next window
        }

        [Fact]
        public void ExecutorAnswersTimeoutWhenBudgetIsSpent()
        {
            long now = 0;
            var budget = new HoldBudget(() => now);
            var bus = new FakeBus { Hook = () => now += 300000 }; // every transfer "takes" 300 ms
            var lk = new FakeLock();
            var conn = new BridgeConnection(new BridgeExecutor(bus, lk, s => { }, budget), new DenyLog(s => { }), new RateLimiter(1000, () => 0));
            string q = Req("{\"op\":\"rcmd\",\"cmd\":160},{\"op\":\"rcmd\",\"cmd\":160}");
            var stream = new ScriptedStream(Frames_(q, q));
            conn.Serve(stream);
            var r = stream.Responses();
            Assert.StartsWith("{\"ok\":true", r[0]);
            Assert.Equal("{\"ok\":false,\"error\":\"timeout\"}", r[1]);
            Assert.Equal(1, lk.Acquired);
        }

        private sealed class ThrowingLock : ISmbusLock
        {
            public int Calls;
            public bool TryAcquire(int ms) { Calls++; throw new UnauthorizedAccessException("denied by ACL"); }
            public void Release() => throw new InvalidOperationException();
        }

        [Fact]
        public void MutexExceptionAnswersTimeoutAndKeepsConnection()
        {
            var log = new List<string>();
            var lk = new ThrowingLock();
            var conn = new BridgeConnection(new BridgeExecutor(new FakeBus(), lk, log.Add), new DenyLog(log.Add));
            var stream = new ScriptedStream(Frames_(Query, Query, Query));
            conn.Serve(stream);
            var r = stream.Responses();
            Assert.Equal(3, r.Count);
            Assert.All(r, x => Assert.Equal("{\"ok\":false,\"error\":\"timeout\"}", x));
            Assert.Equal(3, lk.Calls);
            Assert.Single(log); // logged once
        }

        private sealed class FlakyBus : ISmbus
        {
            public bool Fail;
            public bool Ready() => true;
            public byte ReadByteData(byte addr, byte cmd) { if (Fail) throw new IOException("nack"); return 1; }
            public void WriteByteData(byte addr, byte cmd, byte value) { }
            public void WriteWordData(byte addr, byte cmd, ushort value) { }
            public void WriteBlockData(byte addr, byte cmd, byte[] data) { }
        }

        [Fact]
        public void LazyBusReopensAfterRepeatedErrorsWithBackoff()
        {
            long now = 0;
            var opened = new List<FlakyBus>();
            var lazy = new LazySmbus(() => { var b = new FlakyBus(); opened.Add(b); return b; }, s => { }, () => now, 5000);
            Assert.Equal(1, lazy.ReadByteData(0x71, 0xA0));
            Assert.Single(opened);
            opened[0].Fail = true;
            for (int i = 0; i < LazySmbus.FailuresBeforeReopen; i++) Assert.Throws<IOException>(() => lazy.ReadByteData(0x71, 0xA0));
            Assert.Throws<IOException>(() => lazy.ReadByteData(0x71, 0xA0)); // back-off: not reopened yet
            Assert.Single(opened);
            now = 6000;
            Assert.Equal(1, lazy.ReadByteData(0x71, 0xA0));
            Assert.Equal(2, opened.Count);
        }

        [Fact]
        public void LazyBusSingleNackKeepsHandle()
        {
            int opens = 0;
            var lazy = new LazySmbus(() => { opens++; return new FlakyBus { Fail = true }; }, s => { });
            Assert.Throws<IOException>(() => lazy.ReadByteData(0x71, 0xA0));
            Assert.Throws<IOException>(() => lazy.ReadByteData(0x71, 0xA0));
            Assert.Equal(1, opens);
        }

        [Fact]
        public void ChangeLogSuppressesRepeats()
        {
            var lines = new List<string>();
            var cl = new ChangeLog(lines.Add);
            for (int i = 0; i < 5; i++) cl.Track(() => { cl.Write("ignoring x"); return false; });
            Assert.Single(lines);
            cl.Track(() => { cl.Write("ignoring y"); return false; });
            Assert.Equal(2, lines.Count);
            cl.Track(() => true); // recovered: nothing written
            cl.Track(() => { cl.Write("ignoring y"); return false; });
            Assert.Equal(3, lines.Count);
        }

        [Fact]
        public void ConfigRefusesJunctionedDirectory()
        {
            string root = Path.Combine(Path.GetTempPath(), "casehud-test-" + Guid.NewGuid().ToString("N"));
            string real = Path.Combine(root, "real");
            string link = Path.Combine(root, "link");
            Directory.CreateDirectory(real);
            try
            {
                File.WriteAllText(Path.Combine(real, "sensors.json"), "{\"smbusBridge\":true}");
                var psi = new ProcessStartInfo("cmd.exe", "/c mklink /J \"" + link + "\" \"" + real + "\"") { UseShellExecute = false, CreateNoWindow = true };
                Process.Start(psi).WaitForExit();
                Assert.True(Directory.Exists(link));
                var log = new List<string>();
                Assert.True(BridgeConfig.ReadSmbusBridge(Path.Combine(real, "sensors.json"), log.Add, _ => true, _ => true));
                Assert.False(BridgeConfig.ReadSmbusBridge(Path.Combine(link, "sensors.json"), log.Add, _ => true, _ => true));
                Assert.Contains(log, l => l.Contains("reparse point"));
            }
            finally
            {
                if (Directory.Exists(link)) Directory.Delete(link); // removes the junction only
                if (Directory.Exists(root)) Directory.Delete(root, true);
            }
        }

        [Fact]
        public void ConfigRefusesUntrustedDirectory()
        {
            string dir = Path.Combine(Path.GetTempPath(), "casehud-test-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(dir);
            try
            {
                string p = Path.Combine(dir, "sensors.json");
                File.WriteAllText(p, "{\"smbusBridge\":true}");
                var log = new List<string>();
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, _ => true, _ => false));
                Assert.Contains(log, l => l.Contains("must be owned by an administrator"));
            }
            finally { Directory.Delete(dir, true); }
        }

        [Fact]
        public void DirectoryTrustRules()
        {
            var admins = new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null);
            var system = new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null);
            var users = new SecurityIdentifier(WellKnownSidType.BuiltinUsersSid, null);
            DirectorySecurity Make(SecurityIdentifier owner, SecurityIdentifier sid, FileSystemRights rights)
            {
                var sec = new DirectorySecurity();
                sec.SetOwner(owner);
                sec.AddAccessRule(new FileSystemAccessRule(system, FileSystemRights.FullControl, AccessControlType.Allow));
                sec.AddAccessRule(new FileSystemAccessRule(sid, rights, AccessControlType.Allow));
                return sec;
            }
            // users may add files (the installed layout), but not delete or take over
            Assert.True(BridgeConfig.IsDirectoryTrusted(Make(admins, users, FileSystemRights.CreateFiles | FileSystemRights.ReadAndExecute)));
            Assert.False(BridgeConfig.IsDirectoryTrusted(Make(admins, users, FileSystemRights.Modify)));
            Assert.False(BridgeConfig.IsDirectoryTrusted(Make(admins, users, FileSystemRights.DeleteSubdirectoriesAndFiles)));
            Assert.False(BridgeConfig.IsDirectoryTrusted(Make(users, admins, FileSystemRights.FullControl))); // owned by a user
        }
    }
}
