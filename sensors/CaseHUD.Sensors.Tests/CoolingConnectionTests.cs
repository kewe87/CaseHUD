// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO.Pipes;
using System.Linq;
using System.Threading;
using CaseHUD.Sensors.Cooling;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class CoolingConnectionTests
    {
        private const string Ok = "{\"ok\":true,\"reads\":[]}";
        private const string Denied = "{\"ok\":false,\"error\":\"denied\"}";
        private const string Io = "{\"ok\":false,\"error\":\"io\"}";
        private const string Key = "/lpc/nct6701d/0/control/6";

        private sealed class FakePorts : IControlPorts
        {
            public readonly List<string> Calls = new List<string>();
            public bool Works = true;
            public bool Breaks;
            public bool GivesBack = true;
            public bool Set(string key, int percent)
            {
                Calls.Add("set " + key + " " + percent);
                if (Breaks) throw new InvalidOperationException("half way");
                return Works;
            }
            public bool Release(string key) { Calls.Add("release " + key); return GivesBack; }
            public int Releases => Calls.Count(c => c.StartsWith("release "));
        }

        private static string Hold(string key, int percent) =>
            "{\"v\":1,\"type\":\"hold\",\"key\":\"" + key + "\",\"percent\":" + percent + "}";

        private static List<string> Serve(ControlHolds holds, RateLimiter limiter, params string[] requests)
        {
            var s = new ScriptedStream(Frames_(requests));
            new CoolingConnection(holds, new DenyLog(x => { }), _ => true, limiter).Serve(s);
            return s.Responses().ToList();
        }

        private static List<string> Serve(FakePorts ports, params string[] requests) =>
            Serve(new ControlHolds(ports, x => { }), null, requests);

        [Fact]
        public void HoldsAndGivesBackWhenTheClientEnds()
        {
            var ports = new FakePorts();
            Assert.Equal(new[] { Ok, Ok }, Serve(ports, Hold(Key, 60), Hold(Key, 80)));
            Assert.Equal(new[] { "set " + Key + " 60", "set " + Key + " 80", "release " + Key }, ports.Calls);
        }

        [Theory]
        [InlineData(0)]
        [InlineData(29)]
        [InlineData(101)]
        [InlineData(-1)]
        public void NeverBelowTheFloorOrAboveFull(int percent)
        {
            var ports = new FakePorts();
            Assert.Equal(Denied, Serve(ports, Hold(Key, percent)).Single());
            Assert.Empty(ports.Calls);
        }

        [Theory]
        [InlineData("/gpu-nvidia/0/control/0")]
        [InlineData("/lpc/nct6701d/0/fan/6")]
        [InlineData("/lpc/nct6701d/0/control/6/x")]
        [InlineData("/lpc/nct6701d/0/control/6\\n")]
        [InlineData("/lpc/../control/6")]
        [InlineData("")]
        public void OnlyMainboardHeaders(string key)
        {
            var ports = new FakePorts();
            Assert.Equal(Denied, Serve(ports, Hold(key, 60)).Single());
            Assert.Empty(ports.Calls);
        }

        [Theory]
        [InlineData("{\"v\":1,\"type\":\"setRingBoot\",\"mode\":\"off\"}")]
        [InlineData("{\"v\":2,\"type\":\"hold\",\"key\":\"/lpc/a/0/control/1\",\"percent\":60}")]
        [InlineData("{\"v\":1,\"type\":\"hold\",\"key\":\"/lpc/a/0/control/1\",\"percent\":60,\"x\":1}")]
        [InlineData("{\"v\":1,\"type\":\"hold\",\"key\":\"/lpc/a/0/control/1\",\"percent\":\"60\"}")]
        [InlineData("{\"v\":1,\"type\":\"hold\",\"key\":\"/lpc/a/0/control/1\",\"percent\":60.5}")]
        [InlineData("{\"v\":1,\"addr\":112,\"lockMs\":5,\"ops\":[]}")]
        [InlineData("not json")]
        public void AnythingElseIsDenied(string request)
        {
            var ports = new FakePorts();
            Assert.Equal(Denied, Serve(ports, request).Single());
            Assert.Empty(ports.Calls);
        }

        [Fact]
        public void AHeaderThatCannotBeSetSaysSo()
        {
            var ports = new FakePorts { Works = false };
            Assert.Equal(Io, Serve(ports, Hold(Key, 60)).Single());
            Assert.Equal(new[] { "set " + Key + " 60" }, ports.Calls); // nothing held, nothing to give back
        }

        [Fact]
        public void AnOldClientDoesNotTakeAHeaderFromTheNewOne()
        {
            var ports = new FakePorts();
            var holds = new ControlHolds(ports, x => { });
            var oldOne = new object();
            var newOne = new object();
            Assert.True(holds.Hold(oldOne, Key, 60));
            Assert.True(holds.Hold(newOne, Key, 70));
            holds.Release(oldOne);
            Assert.DoesNotContain("release " + Key, ports.Calls);
            holds.Release(newOne);
            Assert.Contains("release " + Key, ports.Calls);
        }

        [Fact]
        public void EverythingIsGivenBackAtTheEnd()
        {
            var ports = new FakePorts();
            var holds = new ControlHolds(ports, x => { });
            holds.Hold(new object(), Key, 60);
            holds.Hold(new object(), "/lpc/nct6701d/0/control/1", 60);
            holds.Release(null);
            Assert.Equal(2, ports.Calls.Count(c => c.StartsWith("release ")));
        }

        [Fact]
        public void AHeaderTheBoardDidNotGetBackIsTriedAgain()
        {
            var ports = new FakePorts { GivesBack = false };
            var lines = new List<string>();
            var holds = new ControlHolds(ports, lines.Add);
            var owner = new object();
            holds.Hold(owner, Key, 60);
            holds.Release(owner);
            Assert.Equal(1, ports.Releases);
            Assert.DoesNotContain(lines, l => l.Contains("is back with the board"));
            holds.Sweep();
            holds.Sweep();
            Assert.Equal(3, ports.Releases);
            Assert.Single(lines, l => l.Contains("could not be given back"));

            ports.GivesBack = true;
            holds.Sweep();
            Assert.Equal(4, ports.Releases);
            Assert.Single(lines, l => l.Contains("is back with the board"));
            holds.Sweep();
            Assert.Equal(4, ports.Releases);
        }

        [Fact]
        public void ANewHoldEndsTheTriesToGiveBack()
        {
            var ports = new FakePorts { GivesBack = false };
            var holds = new ControlHolds(ports, x => { });
            var first = new object();
            holds.Hold(first, Key, 60);
            holds.Release(first);
            holds.Hold(new object(), Key, 70);
            int before = ports.Releases;
            holds.Sweep();
            Assert.Equal(before, ports.Releases);
        }

        [Fact]
        public void TheEndOfTheServiceAlsoTriesWhatIsStillOwed()
        {
            var ports = new FakePorts { GivesBack = false };
            var holds = new ControlHolds(ports, x => { });
            var owner = new object();
            holds.Hold(owner, Key, 60);
            holds.Release(owner);
            ports.GivesBack = true;
            holds.Release(null);
            Assert.Equal(2, ports.Releases);
        }

        [Fact]
        public void AWriteThatBrokeHalfWayIsGivenBackToo()
        {
            var ports = new FakePorts { Breaks = true };
            var holds = new ControlHolds(ports, x => { });
            var owner = new object();
            Assert.False(holds.Hold(owner, Key, 60));
            holds.Release(owner);
            Assert.Equal(1, ports.Releases);
        }

        [Fact]
        public void AWriteThatFailsDoesNotEndARunningHold()
        {
            var ports = new FakePorts();
            var holds = new ControlHolds(ports, x => { });
            var owner = new object();
            holds.Hold(owner, Key, 60);
            ports.Works = false;
            Assert.False(holds.Hold(owner, Key, 70));
            holds.Release(owner);
            Assert.Equal(1, ports.Releases);
        }

        [Fact]
        public void AHeaderNobodyRepeatsGoesBackWhileTheConnectionLives()
        {
            long now = 0;
            var ports = new FakePorts();
            var holds = new ControlHolds(ports, x => { }, () => now);
            var owner = new object();
            const string other = "/lpc/nct6701d/0/control/1";
            holds.Hold(owner, Key, 60);
            holds.Hold(owner, other, 60);
            now = CoolingConnection.SilentMs - 1;
            holds.Sweep();
            Assert.Equal(0, ports.Releases);
            holds.Hold(owner, other, 60);
            now = CoolingConnection.SilentMs;
            holds.Sweep();
            Assert.Equal(new[] { "release " + Key }, ports.Calls.Where(c => c.StartsWith("release ")));
        }

        [Fact]
        public void AHungClientLosesItsHeaderOnARealPipe()
        {
            var ports = new FakePorts();
            var holds = new ControlHolds(ports, x => { });
            var deny = new DenyLog(x => { });
            string name = "CaseHUD.Cooling.Test." + Guid.NewGuid().ToString("N");
            using (var srv = new NamedPipeBridgeServer(name, s => new CoolingConnection(holds, deny, _ => true).Serve(s), x => { }, idleMs: 300, sweepMs: 100))
            {
                srv.Start();
                using (var c = new NamedPipeClientStream(".", name, PipeDirection.InOut))
                {
                    c.Connect(3000);
                    byte[] f = Frame(Hold(Key, 60));
                    c.Write(f, 0, f.Length);
                    byte[] body;
                    Assert.Equal(BridgeProtocol.ReadResult.Ok, BridgeProtocol.ReadMessage(c, out body));
                    // It stays connected and says nothing more.
                    for (int i = 0; i < 100 && ports.Releases == 0; i++) Thread.Sleep(50);
                    Assert.Equal(1, ports.Releases);
                }
            }
        }

        [Theory]
        [InlineData("/lpc/nct6701d/0/control/6/control/value", "100")]
        [InlineData("/lpc/it8689e/0/control/1/control/value", "100")]
        [InlineData("/lpc/nct6701d/0/control/6/control/mode", "0")]
        [InlineData("/gpu-nvidia/0/control/0/control/value", "0")]
        [InlineData("/lpc/nct6701d/0/fan/6/name", "0")]
        public void HeadersStartAtFullSpeedAndNothingElseChanges(string setting, string want)
        {
            Assert.Equal(want, ControlValues.Start(setting, "0"));
        }

        [Theory]
        [InlineData(60, 60f, 9000, false)]
        [InlineData(60, 62f, 9000, false)]
        [InlineData(60, 75f, 9000, true)]
        [InlineData(60, 45f, 9000, true)]
        [InlineData(60, 75f, 1000, false)] // just written, the reading is older
        public void WhatTheHeaderReallyRunsAtDecides(int wanted, float read, long since, bool rewrite)
        {
            Assert.Equal(rewrite, ControlValues.MustRewrite(wanted, read, since));
        }

        [Fact]
        public void NoReadingIsNoReasonToWrite()
        {
            Assert.False(ControlValues.MustRewrite(60, null, 9000));
        }

        [Fact]
        public void ItsLogLinesSayWhichPipeItIs()
        {
            var lines = new List<string>();
            using (SmbusBridgeHost host = CoolingConnection.CreateHost(new ControlHolds(new FakePorts(), x => { }), _ => true, lines.Add, _ => true))
            {
                host.Start();
            }
            Assert.NotEmpty(lines);
            Assert.All(lines, l => { Assert.Contains("cooling pipe", l); Assert.DoesNotContain("SMBus", l); });
        }

        [Fact]
        public void RateLimits()
        {
            long now = 0;
            var ports = new FakePorts();
            var got = Serve(new ControlHolds(ports, x => { }), new RateLimiter(1, () => now), Hold(Key, 60), Hold(Key, 60));
            Assert.Equal(new[] { Ok, "{\"ok\":false,\"error\":\"timeout\"}" }, got);
        }

        [Fact]
        public void IsItsOwnPipe()
        {
            Assert.Equal("CaseHUD.Cooling", CoolingConnection.PipeName);
            Assert.NotEqual(SmbusBridgeHost.PipeName, CoolingConnection.PipeName);
        }
    }
}
