// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using CaseHUD.Sensors.Cooling;
using Xunit;

namespace CaseHUD.Sensors.Tests
{
    public class HoldJournalTests
    {
        private const string Key = "/lpc/nct6701d/0/control/6";
        private const string Other = "/lpc/nct6701d/0/control/1";

        private sealed class Ports : IControlPorts
        {
            public readonly List<string> Calls;
            public bool Works = true;
            public bool GivesBack = true;
            public Ports(List<string> calls = null) { Calls = calls ?? new List<string>(); }
            public bool Set(string key, int percent) { Calls.Add("set " + key + " " + percent); return Works; }
            public bool Release(string key) { Calls.Add("release " + key); return GivesBack; }
            public int Count(string prefix) => Calls.Count(c => c.StartsWith(prefix));
        }

        private sealed class MemoryJournal : IHoldJournal
        {
            public HoldRecord Stored;
            public bool Fails;
            public int Saves;
            public readonly List<string> Order;
            public MemoryJournal(List<string> order = null) { Order = order ?? new List<string>(); }

            public HoldRecord Load() => Stored;

            public bool Save(HoldRecord r)
            {
                Order.Add("save held=" + string.Join(",", r.Held.OrderBy(k => k)) + " parked=" + string.Join(",", r.Parked.OrderBy(k => k)));
                if (Fails) return false;
                Saves++;
                var copy = new HoldRecord { UptimeMs = Uptime };
                foreach (string k in r.Held) copy.Held.Add(k);
                foreach (string k in r.Parked) copy.Parked.Add(k);
                Stored = copy;
                return true;
            }

            public long Uptime = 5000;
        }

        private static ControlHolds Holds(Ports p, MemoryJournal j, long uptime = 10000) =>
            new ControlHolds(p, x => { }, null, j, () => uptime);

        [Fact]
        public void TheNoteIsWrittenBeforeTheHeaderIs()
        {
            var order = new List<string>();
            var ports = new Ports(order);
            var journal = new MemoryJournal(order);
            var holds = Holds(ports, journal);
            Assert.True(holds.Hold(new object(), Key, 60));
            Assert.Equal(new[] { "save held=" + Key + " parked=", "set " + Key + " 60" }, order);
            // Repeating the duty writes no note again.
            holds.Hold(new object(), Key, 70);
            Assert.Equal(1, journal.Saves);
        }

        [Fact]
        public void WithoutANoteTheHeaderIsNotTouched()
        {
            var ports = new Ports();
            var journal = new MemoryJournal { Fails = true };
            var holds = Holds(ports, journal);
            Assert.False(holds.Hold(new object(), Key, 60));
            Assert.Empty(ports.Calls);
            // Nothing is held, so there is nothing to give back either.
            holds.Release(null);
            Assert.Empty(ports.Calls);
        }

        [Fact]
        public void AHeaderGivenBackIsStruckFromTheNote()
        {
            var ports = new Ports();
            var journal = new MemoryJournal();
            var holds = Holds(ports, journal);
            var owner = new object();
            holds.Hold(owner, Key, 60);
            Assert.Contains(Key, journal.Stored.Held);
            holds.Release(owner);
            Assert.Equal(1, ports.Count("release "));
            Assert.True(journal.Stored.IsEmpty);
        }

        [Fact]
        public void AHeaderTheBoardDidNotTakeStaysOnTheNoteUntilItDoes()
        {
            var ports = new Ports { GivesBack = false };
            var journal = new MemoryJournal();
            var holds = Holds(ports, journal);
            var owner = new object();
            holds.Hold(owner, Key, 60);
            holds.Release(owner);
            Assert.Contains(Key, journal.Stored.Held);
            ports.GivesBack = true;
            holds.Sweep();
            Assert.True(journal.Stored.IsEmpty);
        }

        [Fact]
        public void AServiceThatEndedHardLeavesNoPumpAtAnUnattendedDuty()
        {
            // First run: the header is held and the service dies, so nothing is given back.
            var journal = new MemoryJournal();
            var first = Holds(new Ports(), journal);
            first.Hold(new object(), Key, 40);
            Assert.Contains(Key, journal.Stored.Held);

            // The next start, while the hardware is still being opened: nothing can be written yet.
            var ports = new Ports { Works = false };
            var holds = Holds(ports, journal);
            Assert.Empty(ports.Calls); // the hardware is not open yet: nothing can be written
            holds.Sweep();
            holds.Sweep();
            Assert.Equal(0, ports.Count("release "));

            // The hardware is open: the header goes to full speed, and stays there.
            ports.Works = true;
            holds.Sweep();
            Assert.Equal("set " + Key + " 100", ports.Calls.Last());
            int sets = ports.Count("set ");
            holds.Sweep();
            holds.Sweep();
            Assert.Equal(sets, ports.Count("set "));
            Assert.Equal(0, ports.Count("release "));
            Assert.Contains(Key, journal.Stored.Parked);
            Assert.DoesNotContain(Key, journal.Stored.Held);
        }

        [Fact]
        public void AParkedHeaderIsParkedAgainWhenItsOwnerLetsGo_NotHandedToTheBoard()
        {
            var journal = new MemoryJournal();
            Holds(new Ports(), journal).Hold(new object(), Key, 40);
            var ports = new Ports();
            var holds = Holds(ports, journal);
            holds.Sweep(); // parked
            var owner = new object();
            Assert.True(holds.Hold(owner, Key, 55));
            Assert.Equal("set " + Key + " 55", ports.Calls.Last());
            holds.Release(owner);
            // The library would write back what it found in the chip when it first touched it in this run:
            // the duty the dead service left. So it goes to full speed instead.
            Assert.Equal("set " + Key + " 100", ports.Calls.Last());
            Assert.Equal(0, ports.Count("release "));
            Assert.Contains(Key, journal.Stored.Parked);
        }

        [Fact]
        public void AParkedHeaderIsAlsoKeptAtFullSpeedWhenTheServiceEndsCleanly()
        {
            var journal = new MemoryJournal();
            Holds(new Ports(), journal).Hold(new object(), Key, 40);
            var ports = new Ports();
            var holds = Holds(ports, journal);
            holds.Sweep();
            holds.Hold(new object(), Key, 70);
            holds.Release(null); // the service stops
            Assert.Equal("set " + Key + " 100", ports.Calls.Last());
            Assert.Equal(0, ports.Count("release "));
        }

        [Fact]
        public void HeadersNeverHeldInThisRunAreHandedToTheBoardAsBefore()
        {
            var journal = new MemoryJournal();
            Holds(new Ports(), journal).Hold(new object(), Key, 40);
            var ports = new Ports();
            var holds = Holds(ports, journal);
            holds.Sweep();
            var owner = new object();
            Assert.True(holds.Hold(owner, Other, 60));
            holds.Release(owner);
            Assert.Equal(new[] { "release " + Other }, ports.Calls.Where(c => c.StartsWith("release ")));
        }

        [Fact]
        public void AfterARestartOfThePcTheNoteIsDroppedAndNothingIsWritten()
        {
            var journal = new MemoryJournal { Uptime = 600000 };
            Holds(new Ports(), journal, uptime: 600000).Hold(new object(), Key, 40);
            var ports = new Ports();
            var holds = Holds(ports, journal, uptime: 45000); // the PC has run for 45 s: it started anew
            holds.Sweep();
            holds.Sweep();
            Assert.Empty(ports.Calls);
            Assert.True(journal.Stored.IsEmpty);
        }

        [Fact]
        public void NoNoteMeansNothingToDo()
        {
            var ports = new Ports();
            var holds = Holds(ports, new MemoryJournal());
            holds.Sweep();
            holds.Release(null);
            Assert.Empty(ports.Calls);
            holds = Holds(ports, new MemoryJournal { Stored = new HoldRecord() });
            holds.Sweep();
            Assert.Empty(ports.Calls);
        }

        [Fact]
        public void AHeaderThatCannotBeReachedIsLetGoAfterAWhile()
        {
            var journal = new MemoryJournal();
            Holds(new Ports(), journal).Hold(new object(), Key, 40);
            var ports = new Ports { Works = false };
            var holds = Holds(ports, journal);
            for (int i = 0; i < ControlHolds.ParkGiveUpSweeps + 1; i++) holds.Sweep();
            int sets = ports.Count("set ");
            holds.Sweep();
            holds.Sweep();
            Assert.Equal(sets, ports.Count("set "));
            Assert.True(journal.Stored.IsEmpty);
        }

        [Fact]
        public void ANoteThatCouldNotBeStruckIsStruckByTheNextSweep()
        {
            var ports = new Ports();
            var journal = new MemoryJournal();
            var holds = Holds(ports, journal);
            var owner = new object();
            holds.Hold(owner, Key, 60);
            journal.Fails = true;
            holds.Release(owner);
            Assert.Contains(Key, journal.Stored.Held); // still the old note
            journal.Fails = false;
            holds.Sweep();
            Assert.True(journal.Stored.IsEmpty);
        }

        [Fact]
        public void TheCleanEndOfTheServiceLeavesNoNote()
        {
            var ports = new Ports();
            var journal = new MemoryJournal();
            var holds = Holds(ports, journal);
            holds.Hold(new object(), Key, 60);
            holds.Hold(new object(), Other, 60);
            holds.Release(null);
            Assert.Equal(2, ports.Count("release "));
            Assert.True(journal.Stored.IsEmpty);
        }

        // --- the file ---

        private static FileHoldJournal File_(string path, List<string> log = null, bool trusted = true, long uptime = 7000) =>
            new FileHoldJournal(path, (log ?? new List<string>()).Add, f => trusted, d => trusted, () => uptime);

        private static string TempPath()
        {
            string dir = Path.Combine(Path.GetTempPath(), "casehud-holds-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(dir); // with the ordinary rights of the test user
            return Path.Combine(dir, "cooling-holds.json");
        }

        [Fact]
        public void TheFileKeepsWhatWasWrittenAndWhenTheClockStood()
        {
            string path = TempPath();
            try
            {
                var j = File_(path, uptime: 123456);
                Assert.Null(j.Load());
                var rec = new HoldRecord();
                rec.Held.Add(Key);
                rec.Parked.Add(Other);
                Assert.True(j.Save(rec));
                HoldRecord got = File_(path).Load();
                Assert.Equal(new[] { Key }, got.Held);
                Assert.Equal(new[] { Other }, got.Parked);
                Assert.Equal(123456, got.UptimeMs);
                Assert.Empty(Directory.GetFiles(Path.GetDirectoryName(path), "*.tmp"));
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void AFileOfAnyoneButAnAdministratorCountsAsNothing()
        {
            string path = TempPath();
            try
            {
                var rec = new HoldRecord();
                rec.Held.Add(Key);
                Assert.True(File_(path).Save(rec));
                var log = new List<string>();
                Assert.Null(File_(path, log, trusted: false).Load());
                Assert.Contains(log, l => l.Contains("ignoring"));
                // And it is not written there either.
                Assert.False(File_(path, log, trusted: false).Save(rec));
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Theory]
        [InlineData("")]
        [InlineData("not json")]
        [InlineData("[]")]
        [InlineData("{\"v\":2,\"held\":[\"/lpc/a/0/control/1\"]}")]
        [InlineData("{\"held\":[\"/lpc/a/0/control/1\"]}")]
        public void AFileThatIsNotOursCountsAsNothing(string content)
        {
            string path = TempPath();
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, content);
                Assert.Null(File_(path).Load());
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void OnlyHeadersOfTheMainboardAreTakenFromTheFile()
        {
            string path = TempPath();
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, "{\"v\":1,\"held\":[\"" + Key + "\",\"/gpu-nvidia/0/control/0\",\"..\\\\x\",5],\"parked\":\"" + Other + "\"}");
                HoldRecord got = File_(path).Load();
                Assert.Equal(new[] { Key }, got.Held);
                Assert.Empty(got.Parked);
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void ALargeFileCountsAsNothing()
        {
            string path = TempPath();
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, "{\"v\":1,\"held\":[],\"pad\":\"" + new string('x', FileHoldJournal.MaxBytes) + "\"}");
                Assert.Null(File_(path).Load());
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }

        [Fact]
        public void AServiceThatEndedHardIsFoundThroughTheFile()
        {
            string path = TempPath();
            try
            {
                var ports = new Ports();
                var first = new ControlHolds(ports, x => { }, null, File_(path, uptime: 50000), () => 50000);
                Assert.True(first.Hold(new object(), Key, 40));
                // The process is gone. A new one starts and finds the file.
                var ports2 = new Ports();
                var second = new ControlHolds(ports2, x => { }, null, File_(path, uptime: 52000), () => 52000);
                second.Sweep();
                Assert.Equal(new[] { "set " + Key + " 100" }, ports2.Calls);
                Assert.Equal(new[] { Key }, File_(path).Load().Parked);
            }
            finally { try { Directory.Delete(Path.GetDirectoryName(path), true); } catch { } }
        }
    }

    public class SnapshotClockTests
    {
        [Fact]
        public void NothingIsKnownBeforeTheFirstSnapshot()
        {
            long now = 1000;
            var c = new SnapshotClock(() => now);
            Assert.Null(c.SnapshotAgeMs());
            Assert.Null(c.HardwareAgeMs("/cpu"));
        }

        [Fact]
        public void TheAgeOfTheSnapshotGrowsWhileNothingNewIsBuilt()
        {
            long now = 1000;
            var c = new SnapshotClock(() => now);
            c.Built();
            Assert.Equal(0, c.SnapshotAgeMs());
            now += 2500;
            Assert.Equal(2500, c.SnapshotAgeMs());
            now += 40000; // the update hangs: the answers go on, and say how old they are
            Assert.Equal(42500, c.SnapshotAgeMs());
            c.Built();
            Assert.Equal(0, c.SnapshotAgeMs());
        }

        [Fact]
        public void EachHardwareKnowsWhenItWasLastRead()
        {
            long now = 0;
            var c = new SnapshotClock(() => now);
            c.Updated("/cpu");
            now += 3000;
            c.Updated("/gpu");
            now += 1000;
            Assert.Equal(4000, c.HardwareAgeMs("/cpu"));
            Assert.Equal(1000, c.HardwareAgeMs("/gpu"));
            Assert.Null(c.HardwareAgeMs("/board")); // never read
            now += 60000; // /gpu keeps failing, /cpu goes on
            c.Updated("/cpu");
            Assert.Equal(0, c.HardwareAgeMs("/cpu"));
            Assert.Equal(61000, c.HardwareAgeMs("/gpu"));
        }

        [Fact]
        public void TheHeaderIsNamedAsTheClientExpectsIt()
        {
            Assert.Equal("X-Snapshot-Age-Ms", SnapshotClock.AgeHeader);
        }
    }
}
