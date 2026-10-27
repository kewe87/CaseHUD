// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.IO.Pipes;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Text;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class HostTests
    {
        private sealed class FakePipe : IPipeServer
        {
            public bool Started, Disposed, Alive = true;
            public bool IsAlive => Alive;
            public void Start() => Started = true;
            public void Dispose() => Disposed = true;
        }

        [Fact]
        public void KillSwitchOffCreatesNoPipe()
        {
            int created = 0;
            var log = new List<string>();
            var host = new SmbusBridgeHost(() => false, () => { created++; return new FakePipe(); }, log.Add);
            host.Poll();
            host.Poll();
            Assert.Equal(0, created);
            Assert.False(host.IsRunning);
            Assert.Empty(log);
        }

        [Fact]
        public void EnableAndDisableWithoutRestart()
        {
            bool on = false;
            var pipes = new List<FakePipe>();
            var log = new List<string>();
            var host = new SmbusBridgeHost(() => on, () => { var p = new FakePipe(); pipes.Add(p); return p; }, log.Add);
            host.Poll();
            Assert.Empty(pipes);

            on = true;
            host.Poll();
            host.Poll(); // already running: no second pipe
            Assert.Single(pipes);
            Assert.True(pipes[0].Started);
            Assert.True(host.IsRunning);
            Assert.Contains(log, l => l.Contains("enabled"));

            on = false;
            host.Poll();
            Assert.True(pipes[0].Disposed);
            Assert.False(host.IsRunning);
            Assert.Contains(log, l => l.Contains("disabled"));

            on = true;
            host.Poll();
            Assert.Equal(2, pipes.Count);
            host.Dispose();
            Assert.True(pipes[1].Disposed);
        }

        [Fact]
        public void SettingErrorCountsAsOff()
        {
            var pipe = new FakePipe();
            bool throwNow = false;
            var host = new SmbusBridgeHost(() => { if (throwNow) throw new IOException("x"); return true; }, () => pipe, s => { });
            host.Poll();
            Assert.True(host.IsRunning);
            throwNow = true;
            host.Poll();
            Assert.False(host.IsRunning);
            Assert.True(pipe.Disposed);
        }

        [Fact]
        public void PipeStartFailureIsRetriedAndLoggedOnce()
        {
            int tries = 0;
            var log = new List<string>();
            var host = new SmbusBridgeHost(() => true, () => { tries++; if (tries < 3) throw new IOException("busy"); return new FakePipe(); }, log.Add);
            host.Poll();
            host.Poll();
            Assert.False(host.IsRunning);
            Assert.Single(log);
            host.Poll();
            Assert.True(host.IsRunning);
        }

        [Fact]
        public void ConfigDefaultsToOff()
        {
            var log = new List<string>();
            string dir = Path.Combine(Path.GetTempPath(), "casehud-test-" + Guid.NewGuid().ToString("N"));
            Assert.False(BridgeConfig.ReadSmbusBridge(Path.Combine(dir, "sensors.json"), log.Add)); // no directory
            Directory.CreateDirectory(dir);
            try
            {
                string p = Path.Combine(dir, "sensors.json");
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add)); // no file
                Func<FileSecurity, bool> yes = _ => true;
                Func<DirectorySecurity, bool> dyes = _ => true;

                File.WriteAllText(p, "{}");
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, yes, dyes));
                File.WriteAllText(p, "{\"smbusBridge\":false}");
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, yes, dyes));
                File.WriteAllText(p, "{\"smbusBridge\":\"true\"}");
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, yes, dyes));
                File.WriteAllText(p, "{broken");
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, yes, dyes));
                File.WriteAllText(p, "{\"other\":1,\"smbusBridge\":true}");
                Assert.True(BridgeConfig.ReadSmbusBridge(p, log.Add, yes, dyes));

                // a file the current (non-admin-owned) user may write is never trusted
                Assert.False(BridgeConfig.ReadSmbusBridge(p, log.Add, _ => false, dyes));
                Assert.Contains(log, l => l.Contains("ignoring"));
            }
            finally
            {
                Directory.Delete(dir, true);
            }
        }

        [Fact]
        public void ConfigTrustRules()
        {
            var admins = new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null);
            var system = new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null);
            var users = new SecurityIdentifier(WellKnownSidType.BuiltinUsersSid, null);

            FileSecurity Make(SecurityIdentifier owner, params (SecurityIdentifier sid, FileSystemRights rights)[] rules)
            {
                var sec = new FileSecurity();
                sec.SetOwner(owner);
                foreach (var r in rules) sec.AddAccessRule(new FileSystemAccessRule(r.sid, r.rights, AccessControlType.Allow));
                return sec;
            }

            Assert.True(BridgeConfig.IsTrusted(Make(admins, (system, FileSystemRights.FullControl), (admins, FileSystemRights.FullControl), (users, FileSystemRights.ReadAndExecute))));
            Assert.False(BridgeConfig.IsTrusted(Make(admins, (users, FileSystemRights.Write))));
            Assert.False(BridgeConfig.IsTrusted(Make(admins, (users, FileSystemRights.Modify))));
            Assert.False(BridgeConfig.IsTrusted(Make(users, (admins, FileSystemRights.FullControl)))); // wrong owner
        }

        [Fact]
        public void PipeSecurityAllowsInteractiveAndDeniesNetwork()
        {
            PipeSecurity sec = NamedPipeBridgeServer.CreateSecurity();
            var rules = sec.GetAccessRules(true, false, typeof(SecurityIdentifier));
            bool Has(string sid, AccessControlType t)
            {
                foreach (PipeAccessRule r in rules) if (r.IdentityReference.Value == sid && r.AccessControlType == t) return true;
                return false;
            }
            Assert.True(Has("S-1-5-4", AccessControlType.Allow));
            Assert.True(Has("S-1-5-32-544", AccessControlType.Allow));
            Assert.True(Has("S-1-5-18", AccessControlType.Allow));
            Assert.True(Has("S-1-5-2", AccessControlType.Deny));
            Assert.True(Has("S-1-5-7", AccessControlType.Deny));
        }

        [Fact]
        public void RealNamedPipeRoundTripAndDisable()
        {
            string name = "CaseHUD.Smbus.Test." + Guid.NewGuid().ToString("N");
            var bus = new FakeBus();
            var exec = new BridgeExecutor(bus, new FakeLock(), s => { });
            var deny = new DenyLog(s => { });
            bool on = true;
            var host = new SmbusBridgeHost(() => on,
                () => new NamedPipeBridgeServer(name, s => new BridgeConnection(exec, deny).Serve(s), s => { }), s => { });
            host.Poll();
            try
            {
                using (var c = new NamedPipeClientStream(".", name, PipeDirection.InOut))
                {
                    c.Connect(3000);
                    for (int i = 0; i < 3; i++) // one connection, several batches
                    {
                        byte[] f = Frame(Req("{\"op\":\"rcmd\",\"cmd\":161}"));
                        c.Write(f, 0, f.Length);
                        byte[] body;
                        Assert.Equal(BridgeProtocol.ReadResult.Ok, BridgeProtocol.ReadMessage(c, out body));
                        Assert.Equal("{\"ok\":true,\"reads\":[90]}", Encoding.UTF8.GetString(body));
                    }

                    on = false;
                    host.Poll(); // disabling drops the open connection and removes the pipe
                    byte[] g = Frame(Req("{\"op\":\"rcmd\",\"cmd\":161}"));
                    byte[] after = null;
                    try
                    {
                        c.Write(g, 0, g.Length);
                        BridgeProtocol.ReadMessage(c, out after);
                    }
                    catch (IOException) { }
                    Assert.Null(after);
                }
                using (var c = new NamedPipeClientStream(".", name, PipeDirection.InOut))
                {
                    Assert.Throws<TimeoutException>(() => c.Connect(300));
                }
            }
            finally
            {
                host.Dispose();
            }
        }
    }
}
