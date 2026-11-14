// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Linq;
using CaseHUD.Sensors.Cooling;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class ClientImageTests
    {
        private const string Dir = @"C:\Program Files\CaseHUD";
        private static readonly string[] Names = ClientImage.CoolingClients;

        private sealed class Ports : IControlPorts
        {
            public readonly List<string> Calls = new List<string>();
            public bool Set(string key, int percent) { Calls.Add("set " + key + " " + percent); return true; }
            public bool Release(string key) { Calls.Add("release " + key); return true; }
        }

        private static Func<int, bool> Policy(Dictionary<int, string> images) =>
            ClientImage.ForInstall(Dir, Names, pid => images.TryGetValue(pid, out string p) ? (p == "throw" ? throw new InvalidOperationException() : p) : null);

        [Fact]
        public void TheProgramAndTheGuardOfTheInstallationAreAccepted()
        {
            var ok = Policy(new Dictionary<int, string>
            {
                [1] = Dir + @"\CaseHUD.exe",
                [2] = Dir + @"\CaseHUD-Guard.exe",
                [3] = Dir.ToLowerInvariant() + @"\casehud.exe",
                [4] = Dir + @"\.\CaseHUD.exe",
            });
            Assert.All(new[] { 1, 2, 3, 4 }, pid => Assert.True(ok(pid), "pid " + pid));
        }

        [Fact]
        public void EverythingElseIsRefused()
        {
            var no = Policy(new Dictionary<int, string>
            {
                [1] = @"C:\Users\other\AppData\Local\Temp\CaseHUD.exe",
                [2] = Dir + @"\sensors\CaseHUD.exe",
                [3] = Dir + @"\CaseHUD.Sensors.exe",
                [4] = Dir + @"\Other.exe",
                [5] = @"C:\Program Files\CaseHUD2\CaseHUD.exe",
                [6] = Dir + @"\CaseHUD.exe.bak",
                [7] = "throw",
                [8] = "",
            });
            Assert.All(new[] { 0, -1, 1, 2, 3, 4, 5, 6, 7, 8, 99 }, pid => Assert.False(no(pid), "pid " + pid));
        }

        [Fact]
        public void WithoutAnInstallationNothingIsAccepted()
        {
            var none = ClientImage.ForInstall(null, Names, _ => Dir + @"\CaseHUD.exe");
            Assert.False(none(1));
        }

        [Fact]
        public void TheImageOfThisProcessCanBeRead()
        {
            string image = ClientImage.ImageOf(Process.GetCurrentProcess().Id);
            Assert.False(string.IsNullOrEmpty(image));
            Assert.True(File.Exists(image));
            Assert.Null(ClientImage.ImageOf(int.MaxValue));
        }

        [Fact]
        public void AProgramThatIsNotOfTheInstallationIsRefusedOnARealPipe()
        {
            // The test host is not in the directory the policy names.
            Func<int, bool> policy = ClientImage.ForInstall(Dir, Names, ClientImage.ImageOf);
            var ports = new Ports();
            var holds = new ControlHolds(ports, x => { });
            var deny = new DenyLog(x => { });
            var log = new List<string>();
            string name = "CaseHUD.Cooling.Test." + Guid.NewGuid().ToString("N");
            using (var srv = new NamedPipeBridgeServer(name, s => new CoolingConnection(holds, deny, _ => true).Serve(s), log.Add, clientAllowed: policy))
            {
                srv.Start();
                using (var c = new NamedPipeClientStream(".", name, PipeDirection.InOut))
                {
                    c.Connect(3000);
                    byte[] body = null;
                    BridgeProtocol.ReadResult r;
                    try
                    {
                        byte[] f = Frame("{\"v\":1,\"type\":\"hold\",\"key\":\"/lpc/nct6701d/0/control/1\",\"percent\":60}");
                        c.Write(f, 0, f.Length);
                        r = BridgeProtocol.ReadMessage(c, out body);
                    }
                    catch (IOException) { r = BridgeProtocol.ReadResult.Closed; }
                    Assert.NotEqual(BridgeProtocol.ReadResult.Ok, r);
                }
                Assert.Empty(ports.Calls);
                Assert.Contains(log, l => l.Contains("refused a client"));
            }
        }

        [Fact]
        public void AProgramOfTheInstallationIsServedOnARealPipe()
        {
            // The directory of this very test host is the installation.
            string exe = Process.GetCurrentProcess().MainModule.FileName;
            Func<int, bool> policy = ClientImage.ForInstall(Path.GetDirectoryName(exe), new[] { Path.GetFileName(exe) }, ClientImage.ImageOf);
            var ports = new Ports();
            var holds = new ControlHolds(ports, x => { });
            var deny = new DenyLog(x => { });
            string name = "CaseHUD.Cooling.Test." + Guid.NewGuid().ToString("N");
            using (var srv = new NamedPipeBridgeServer(name, s => new CoolingConnection(holds, deny, _ => true).Serve(s), x => { }, clientAllowed: policy))
            {
                srv.Start();
                using (var c = new NamedPipeClientStream(".", name, PipeDirection.InOut))
                {
                    c.Connect(3000);
                    byte[] f = Frame("{\"v\":1,\"type\":\"hold\",\"key\":\"/lpc/nct6701d/0/control/1\",\"percent\":60}");
                    c.Write(f, 0, f.Length);
                    byte[] body;
                    Assert.Equal(BridgeProtocol.ReadResult.Ok, BridgeProtocol.ReadMessage(c, out body));
                }
            }
        }
    }
}
