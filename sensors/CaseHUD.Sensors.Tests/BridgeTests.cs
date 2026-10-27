// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using CaseHUD.Sensors.Smbus;
using Xunit;
using static CaseHUD.Sensors.Tests.Frames;

namespace CaseHUD.Sensors.Tests
{
    public class BridgeTests
    {
        private sealed class Rig
        {
            public readonly FakeBus Bus = new FakeBus();
            public readonly FakeLock Lock = new FakeLock();
            public readonly List<string> Log = new List<string>();
            public long Now = 0;
            public readonly BridgeExecutor Exec;

            // verified = true: pretend the ENE identification already passed for every address
            public Rig(bool verified = true)
            {
                Exec = new BridgeExecutor(Bus, Lock, Log.Add);
                if (verified) for (byte a = 0x70; a <= 0x77; a++) Exec.Verifier.Mark(a);
            }

            public List<string> Run(byte[] input, int chunk = int.MaxValue, double perSecond = BridgeConnection.BatchesPerSecond)
            {
                var exec = Exec;
                var conn = new BridgeConnection(exec, new DenyLog(Log.Add, () => Now), new RateLimiter(perSecond, () => Now));
                var s = new ScriptedStream(input, chunk);
                conn.Serve(s);
                return s.Responses();
            }

            public string One(string json) => Run(Frame(json)).Single();
        }

        private const string Denied = "{\"ok\":false,\"error\":\"denied\"}";
        private const string Timeout = "{\"ok\":false,\"error\":\"timeout\"}";
        private const string Io = "{\"ok\":false,\"error\":\"io\"}";

        [Theory]
        [InlineData("80")]   // 0x50 SPD
        [InlineData("111")]  // 0x6F
        [InlineData("120")]  // 0x78
        public void RejectsForbiddenAddress(string addr)
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":1}", addr: addr)));
            Assert.Empty(r.Bus.Calls);
            Assert.Equal(0, r.Lock.Acquired);
        }

        [Theory]
        [InlineData("112")] // 0x70
        [InlineData("119")] // 0x77
        public void AcceptsAddressRange(string addr)
        {
            var r = new Rig();
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rb\",\"reg\":1}", addr: addr)));
        }

        [Theory]
        [InlineData("0")]
        [InlineData("51")]
        [InlineData("-1")]
        [InlineData("1.5")]
        public void RejectsLockMsOutOfRange(string lockMs)
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":1}", lockMs: lockMs)));
            Assert.Empty(r.Bus.Calls);
        }

        [Fact]
        public void AcceptsLockMsLimits()
        {
            var r = new Rig();
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rb\"}", lockMs: "1")));
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rb\"}", lockMs: "50")));
            Assert.Equal(50, r.Lock.LastTimeout);
        }

        [Fact]
        public void OpCountLimits()
        {
            var r = new Rig();
            string op = "{\"op\":\"rcmd\",\"cmd\":160}";
            Assert.Equal(Denied, r.One(Req(string.Join(",", Enumerable.Repeat(op, 65)))));
            Assert.Equal(Denied, r.One(Req("")));
            Assert.Empty(r.Bus.Calls);
            string ok = r.One(Req(string.Join(",", Enumerable.Repeat(op, 64))));
            Assert.Equal(64, System.Text.RegularExpressions.Regex.Matches(ok, "90").Count);
        }

        [Fact]
        public void BlockSizeLimits()
        {
            var r = new Rig();
            string b33 = Convert.ToBase64String(new byte[33]);
            string b32 = Convert.ToBase64String(new byte[32]);
            Assert.Equal(Denied, r.One(Req("{\"op\":\"blk\",\"reg\":33024,\"data\":\"" + b33 + "\"}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"blk\",\"reg\":33024,\"data\":\"\"}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"blk\",\"reg\":33024}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"blk\",\"reg\":33024,\"data\":\"!!notbase64\"}")));
            Assert.Empty(r.Bus.Calls);
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"blk\",\"reg\":33024,\"data\":\"" + b32 + "\"}")));
        }

        [Theory]
        [InlineData(159)] // 0x9F
        [InlineData(176)] // 0xB0
        [InlineData(0)]
        public void RejectsRawCommandOutsideIdentRange(int cmd)
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rcmd\",\"cmd\":" + cmd + "}")));
            Assert.Empty(r.Bus.Calls);
        }

        [Fact]
        public void RejectsUnknownOpsAndOutOfRangeFields()
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One(Req("{\"op\":\"ww\",\"reg\":1,\"val\":2}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"\"}")));
            Assert.Equal(Denied, r.One(Req("{}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":65536}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"wb\",\"reg\":1,\"val\":256}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":-1}")));
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":\"1\"}")));
            Assert.Equal(Denied, r.One(Req("5")));
            Assert.Empty(r.Bus.Calls);
            // one bad op rejects the whole batch before anything runs
            Assert.Equal(Denied, r.One(Req("{\"op\":\"wb\",\"reg\":1,\"val\":2},{\"op\":\"ww\"}")));
            Assert.Empty(r.Bus.Calls);
        }

        [Theory]
        [InlineData("0")]
        [InlineData("2")]
        public void RejectsWrongVersion(string v)
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\"}", v: v)));
            Assert.Equal(Denied, r.One("{\"addr\":113,\"lockMs\":10,\"ops\":[{\"op\":\"rb\"}]}")); // v missing
            Assert.Empty(r.Bus.Calls);
        }

        [Fact]
        public void RejectsMalformedJson()
        {
            var r = new Rig();
            Assert.Equal(Denied, r.One("{not json"));
            Assert.Equal(Denied, r.One("ÿþ"));
            Assert.Empty(r.Bus.Calls);
        }

        [Fact]
        public void OversizeMessageClosesConnection()
        {
            var r = new Rig();
            // length prefix 64 KiB + 1: no response, nothing further is read
            byte[] input = BitConverter.GetBytes((uint)(BridgeProtocol.MaxMessage + 1))
                .Concat(Frame(Req("{\"op\":\"rb\"}"))).ToArray();
            Assert.Empty(r.Run(input));
            Assert.Empty(r.Bus.Calls);
            Assert.Contains(r.Log, l => l.Contains("size"));
            // zero length also drops the connection
            Assert.Empty(r.Run(new byte[4]));
            // exactly the limit is a valid size (but not valid JSON here)
            var big = new byte[4 + BridgeProtocol.MaxMessage];
            BitConverter.GetBytes((uint)BridgeProtocol.MaxMessage).CopyTo(big, 0);
            Assert.Equal(new[] { Denied }, r.Run(big));
        }

        [Fact]
        public void EneCommandSequences()
        {
            var r = new Rig();
            r.Bus.ReadQueue.Enqueue(0x11);
            r.Bus.ReadQueue.Enqueue(0x22);
            r.Bus.ReadQueue.Enqueue(0x33);
            string data = Convert.ToBase64String(new byte[] { 1, 2, 3 });
            string resp = r.One(Req(
                "{\"op\":\"rb\",\"reg\":4660}," +                      // 0x1234
                "{\"op\":\"wb\",\"reg\":32800,\"val\":1}," +           // 0x8020
                "{\"op\":\"blk\",\"reg\":33024,\"data\":\"" + data + "\"}," + // 0x8100
                "{\"op\":\"rcmd\",\"cmd\":163}," +                     // 0xA3
                "{\"op\":\"rb\"}"));                                   // reg 0 (omitted)
            Assert.Equal("{\"ok\":true,\"reads\":[17,34,51]}", resp);
            Assert.Equal(new[]
            {
                "ready",
                "wwd 71 00 3412", "rbd 71 81",
                "wwd 71 00 2080", "wbd 71 01 01",
                "wwd 71 00 0081", "wblk 71 03 010203",
                "rbd 71 a3",
                "wwd 71 00 0000", "rbd 71 81",
            }, r.Bus.Calls.ToArray());
            Assert.Equal(1, r.Lock.Acquired);
            Assert.Equal(1, r.Lock.Released);
            Assert.Equal(10, r.Lock.LastTimeout);
        }

        [Fact]
        public void ReadsKeepOpOrderAndAllByteValues()
        {
            var r = new Rig();
            foreach (byte b in new byte[] { 0, 255, 7 }) r.Bus.ReadQueue.Enqueue(b);
            Assert.Equal("{\"ok\":true,\"reads\":[0,255,7]}",
                r.One(Req("{\"op\":\"rcmd\",\"cmd\":160},{\"op\":\"wb\",\"reg\":32801},{\"op\":\"rcmd\",\"cmd\":161},{\"op\":\"rb\",\"reg\":2}")));
        }

        [Fact]
        public void WritesOnlyBatchHasEmptyReads()
        {
            var r = new Rig();
            Assert.Equal("{\"ok\":true,\"reads\":[]}", r.One(Req("{\"op\":\"wb\",\"reg\":32801,\"val\":9}")));
        }

        [Fact]
        public void TimeoutWhenMutexHeld()
        {
            var r = new Rig { Lock = { Held = true } };
            Assert.Equal(Timeout, r.One(Req("{\"op\":\"rb\"}", lockMs: "20")));
            Assert.Empty(r.Bus.Calls);
            Assert.Equal(0, r.Lock.Released);
            Assert.Equal(20, r.Lock.LastTimeout);
            r.Lock.Held = false;
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rb\"}")));
        }

        [Fact]
        public void TimeoutWhenAnotherPortIsSelected()
        {
            var r = new Rig();
            r.Bus.ReadyResult = false;
            Assert.Equal(Timeout, r.One(Req("{\"op\":\"rb\"}")));
            Assert.Equal(new[] { "ready" }, r.Bus.Calls.ToArray());
            Assert.Equal(1, r.Lock.Released);
        }

        [Fact]
        public void IoErrorAbortsBatchAndReleasesMutex()
        {
            var r = new Rig();
            r.Bus.FailAt = 2; // wwd, rbd ok; the third transfer fails
            string resp = r.One(Req("{\"op\":\"rb\",\"reg\":1},{\"op\":\"wb\",\"reg\":32802,\"val\":3},{\"op\":\"rb\",\"reg\":4}"));
            Assert.Equal(Io, resp);
            Assert.Equal(3, r.Bus.Calls.Count - 1); // no transfer after the failing one
            Assert.Equal(1, r.Lock.Released);
        }

        // ---- ENE verification and register allow-list ----

        private const string Wb = "{\"op\":\"wb\",\"reg\":32801,\"val\":1}";

        private static void MakeEne(FakeBus bus, string name)
        {
            bus.Ene = true;
            bus.EneName = name;
        }

        [Fact]
        public void UnverifiedAddressDeniesWritesWithoutWritingAnything()
        {
            var r = new Rig(verified: false);
            r.Bus.Ene = true;
            r.Bus.EneName = "SOMETHING-ELSE"; // not an ENE name
            Assert.Equal(Denied, r.One(Req(Wb)));
            // only read transfers (ident and name read); the register select is the only write-like transfer
            Assert.DoesNotContain(r.Bus.Calls, c => c.StartsWith("wbd") || c.StartsWith("wblk"));
            Assert.Equal(1, r.Lock.Released);
        }

        [Fact]
        public void NonAnsweringIdentDeniesWithoutSelectingRegisters()
        {
            var r = new Rig(verified: false); // fake bus answers 0x5A everywhere: not an ENE ident
            Assert.Equal(Denied, r.One(Req("{\"op\":\"rb\",\"reg\":4096}")));
            Assert.DoesNotContain(r.Bus.Calls, c => c.StartsWith("wwd") || c.StartsWith("wbd") || c.StartsWith("wblk"));
        }

        [Fact]
        public void IdentOnlyBatchNeedsNoVerification()
        {
            var r = new Rig(verified: false);
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rcmd\",\"cmd\":160}")));
            Assert.Equal(new[] { "ready", "rbd 71 a0" }, r.Bus.Calls.ToArray());
        }

        [Fact]
        public void VerifiesOnceAndCaches()
        {
            var r = new Rig(verified: false);
            MakeEne(r.Bus, "AUDA0-E6K5-0101");
            Assert.StartsWith("{\"ok\":true", r.One(Req(Wb)));
            int first = r.Bus.Calls.Count;
            Assert.True(first > 5);
            Assert.StartsWith("{\"ok\":true", r.One(Req(Wb)));
            Assert.Equal(first + 3, r.Bus.Calls.Count); // ready, register select, write
        }

        [Fact]
        public void ErrorForcesReverification()
        {
            var r = new Rig(verified: false);
            MakeEne(r.Bus, "AUDA0-E6K5-0101");
            Assert.StartsWith("{\"ok\":true", r.One(Req(Wb)));
            Assert.True(r.Exec.Verifier.IsVerified(0x71));
            r.Bus.FailAt = r.Bus.Calls.Count; // the next transfer fails
            Assert.Equal(Io, r.One(Req(Wb)));
            Assert.False(r.Exec.Verifier.IsVerified(0x71));
            r.Bus.FailAt = -1;
            r.Bus.Calls.Clear();
            Assert.StartsWith("{\"ok\":true", r.One(Req(Wb)));
            Assert.Contains(r.Bus.Calls, c => c == "rbd 71 a0"); // identified again
        }

        [Theory]
        [InlineData(32800, 1, true)]   // 0x8020 direct
        [InlineData(32803, 7, true)]   // 0x8023
        [InlineData(32804, 1, false)]  // 0x8024
        [InlineData(33008, 1, false)]  // 0x80F0
        [InlineData(32928, 1, true)]   // 0x80A0 apply
        [InlineData(32928, 170, false)] // 0xAA = save to EEPROM
        [InlineData(32928, 0, false)]
        [InlineData(32815, 1, true)]   // 0x802F apply (v1)
        [InlineData(32815, 170, false)]
        [InlineData(4096, 1, false)]   // device name area
        [InlineData(0, 1, false)]
        public void RegisterWriteAllowList(int reg, int val, bool allowed)
        {
            var r = new Rig();
            string resp = r.One(Req("{\"op\":\"wb\",\"reg\":" + reg + ",\"val\":" + val + "}"));
            if (allowed) Assert.StartsWith("{\"ok\":true", resp);
            else
            {
                Assert.Equal(Denied, resp);
                Assert.Empty(r.Bus.Calls);
            }
        }

        [Theory]
        [InlineData(32768, 15, true)]   // 0x8000 v1 colors
        [InlineData(32768, 32, true)]
        [InlineData(32768, 33, false)]  // runs into the direct register
        [InlineData(32784, 17, false)]
        [InlineData(33024, 32, true)]   // 0x8100
        [InlineData(33024 + 40, 25, false)]
        [InlineData(33120, 32, true)]   // 0x8160
        [InlineData(33152, 32, true)]   // 0x8180 .. 0x81A0
        [InlineData(33153, 32, false)]
        [InlineData(32928, 1, false)]   // 0x80A0
        [InlineData(4096, 16, false)]
        public void BlockWriteAllowList(int reg, int len, bool allowed)
        {
            var r = new Rig();
            string data = Convert.ToBase64String(new byte[len > 32 ? 32 : len]);
            if (len > 32) return; // size limit is covered elsewhere
            string resp = r.One(Req("{\"op\":\"blk\",\"reg\":" + reg + ",\"data\":\"" + data + "\"}"));
            if (allowed) Assert.StartsWith("{\"ok\":true", resp);
            else
            {
                Assert.Equal(Denied, resp);
                Assert.Empty(r.Bus.Calls);
            }
        }

        [Fact]
        public void ReadsOfAnyRegisterAreFine()
        {
            var r = new Rig();
            Assert.StartsWith("{\"ok\":true", r.One(Req("{\"op\":\"rb\",\"reg\":4096},{\"op\":\"rb\",\"reg\":65535}")));
        }

        [Fact]
        public void ConnectionServesManyBatches()
        {
            var r = new Rig();
            string q = Req("{\"op\":\"rcmd\",\"cmd\":160}");
            var res = r.Run(Frames_(q, q, "bad", q));
            Assert.Equal(4, res.Count);
            Assert.Equal(Denied, res[2]);
            Assert.StartsWith("{\"ok\":true", res[3]);
        }

        [Fact]
        public void RateLimitAnswersTimeout()
        {
            var r = new Rig(); // time stands still: only the burst is allowed
            string q = Req("{\"op\":\"rcmd\",\"cmd\":160}");
            var res = r.Run(Frames_(Enumerable.Repeat(q, 130).ToArray()));
            Assert.Equal(130, res.Count);
            Assert.Equal(120, res.Count(x => x.StartsWith("{\"ok\":true")));
            Assert.Equal(10, res.Count(x => x == Timeout));
            Assert.Equal(120, r.Lock.Acquired);
        }

        [Fact]
        public void RateLimitRefills()
        {
            long now = 0;
            var rl = new RateLimiter(120, () => now);
            for (int i = 0; i < 120; i++) Assert.True(rl.Allow());
            Assert.False(rl.Allow());
            now += 100; // 100 ms = 12 tokens
            int ok = 0;
            while (rl.Allow()) ok++;
            Assert.Equal(12, ok);
        }

        [Fact]
        public void DenyLogIsRateLimited()
        {
            var lines = new List<string>();
            long now = 0;
            var d = new DenyLog(lines.Add, () => now, 10000);
            for (int i = 0; i < 50; i++) d.Denied("x");
            Assert.Single(lines);
            now = 10001;
            d.Denied("y");
            Assert.Equal(2, lines.Count);
            Assert.Contains("+49 more", lines[1]);
        }

        [Theory]
        [InlineData(1)]
        [InlineData(3)]
        [InlineData(7)]
        public void FramingHandlesPartialReads(int chunk)
        {
            var r = new Rig();
            string q = Req("{\"op\":\"rcmd\",\"cmd\":160}");
            var res = r.Run(Frames_(q, q), chunk);
            Assert.Equal(2, res.Count);
            Assert.All(res, x => Assert.StartsWith("{\"ok\":true", x));
        }

        [Fact]
        public void TruncatedMessageEndsConnectionWithoutResponse()
        {
            var r = new Rig();
            byte[] f = Frame(Req("{\"op\":\"rcmd\",\"cmd\":160}"));
            Assert.Empty(r.Run(f.Take(f.Length - 3).ToArray()));
            Assert.Empty(r.Run(new byte[] { 5, 0 })); // half a header
            Assert.Empty(r.Bus.Calls);
        }

        [Fact]
        public void ResponseHasLittleEndianLengthPrefix()
        {
            var ms = new MemoryStream();
            BridgeProtocol.WriteMessage(ms, Encoding.UTF8.GetBytes(new string('a', 300)));
            byte[] b = ms.ToArray();
            Assert.Equal(new byte[] { 0x2C, 0x01, 0, 0 }, b.Take(4).ToArray());
            Assert.Equal(304, b.Length);
        }
    }
}
