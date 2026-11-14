// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;

namespace CaseHUD.Sensors.Cooling
{
    /// <summary>
    /// Who holds which header. A header stays set only as long as the connection that
    /// set it lives; then the board's own regulation takes over again.
    ///
    /// A header the service wrote is noted in a journal before it is written. If the
    /// service ends without giving its headers back (killed, crashed, power loss), the
    /// chip keeps the last duty, and the next start finds the note. The hardware library
    /// can only hand a header back to the board if it saw it before the first write in
    /// the same run, so a restarted service cannot do that: it sets such a header to full
    /// speed ("parked") and keeps it there until the PC restarts, which resets the chip. A
    /// pump therefore never stays at a low duty that nobody looks after.
    /// </summary>
    internal sealed class ControlHolds
    {
        /// <summary>The duty of a parked header.</summary>
        public const int ParkPercent = 100;

        /// <summary>Sweeps (2 s each) after which a header that cannot be parked is let go: 15 minutes.</summary>
        public const int ParkGiveUpSweeps = 450;

        private sealed class Holding
        {
            public object Owner;
            public long HeardAt;
        }

        private readonly IControlPorts _ports;
        private readonly Action<string> _log;
        private readonly Func<long> _now;
        private readonly IHoldJournal _journal;
        private readonly Func<long> _uptime;
        private readonly object _sync = new object();
        private readonly Dictionary<string, Holding> _held = new Dictionary<string, Holding>();
        private readonly HashSet<string> _owed = new HashSet<string>(); // given up, but the board does not have them yet
        private readonly HashSet<string> _parked = new HashSet<string>(StringComparer.Ordinal); // end at full speed, not at the board
        private readonly Dictionary<string, int> _parkDue = new Dictionary<string, int>(StringComparer.Ordinal); // parked is not written yet; sweeps tried
        private string _saved = ""; // what the journal holds
        private bool _unsaved; // the journal is behind and is written again by the next sweep

        public ControlHolds(IControlPorts ports, Action<string> log, Func<long> nowMs = null, IHoldJournal journal = null, Func<long> uptimeMs = null)
        {
            _ports = ports;
            _log = log;
            _now = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() / (System.Diagnostics.Stopwatch.Frequency / 1000));
            _journal = journal;
            _uptime = uptimeMs ?? Uptime.Ms;
            if (_journal != null) Recover(_journal.Load());
        }

        /// <summary>Headers of a run that ended without giving them back, as the journal knows them.</summary>
        private void Recover(HoldRecord rec)
        {
            if (rec == null || rec.IsEmpty) return;
            _saved = Describe(rec);
            if (_uptime() < rec.UptimeMs)
            {
                // The PC started anew since: the chip is back to what the board makes of it.
                _log("cooling pipe: the PC restarted since headers were held; nothing to take back");
                Journal(null);
                return;
            }
            foreach (string key in rec.Held) Park(key);
            foreach (string key in rec.Parked) Park(key);
            _log("cooling pipe: " + _parked.Count + " header(s) were held by a run that ended without giving them back; " +
                 "they are set to full speed and stay there until the PC restarts");
            Journal(null);
        }

        private void Park(string key)
        {
            _parked.Add(key);
            if (!_parkDue.ContainsKey(key)) _parkDue[key] = 0;
        }

        public bool Hold(object owner, string key, int percent)
        {
            lock (_sync)
            {
                bool had = _held.ContainsKey(key);
                // The note first: a write that no note tells of cannot be taken back after a crash. Without a
                // note the board keeps the header.
                if (!had && !Journal(key))
                {
                    _log("cooling pipe: " + key + " is not held, the journal cannot be written");
                    return false;
                }
                // Before the write: if it fails half way, the end of this owner still gives the header back.
                // The newest connection wins, an old one may just not have ended yet.
                _held[key] = new Holding { Owner = owner, HeardAt = _now() };
                bool done;
                try
                {
                    done = _ports.Set(key, percent);
                }
                catch (Exception)
                {
                    return false;
                }
                if (!done)
                {
                    if (!had)
                    {
                        _held.Remove(key);
                        Journal(null);
                    }
                    return false;
                }
                _owed.Remove(key);
                _parkDue.Remove(key); // someone sets it now; it is parked again when they let go
                if (!had) _log("cooling pipe: holding " + key);
                return true;
            }
        }

        /// <summary>Gives back what owner holds; with null, everything.</summary>
        public void Release(object owner)
        {
            lock (_sync)
            {
                foreach (string key in new List<string>(_held.Keys))
                {
                    if (owner == null || ReferenceEquals(_held[key].Owner, owner)) GiveBack(key);
                }
                if (owner == null) RetryOwed();
                Journal(null);
            }
        }

        /// <summary>
        /// Runs from a timer: a header nobody repeats any more goes back, even if its
        /// connection is still alive, and one that could not be given back is tried again.
        /// </summary>
        public void Sweep()
        {
            lock (_sync)
            {
                long now = _now();
                foreach (string key in new List<string>(_held.Keys))
                {
                    if (now - _held[key].HeardAt >= CoolingConnection.SilentMs) GiveBack(key);
                }
                RetryOwed();
                if (_unsaved || Describe(Build()) != _saved) Journal(null);
            }
        }

        private void GiveBack(string key)
        {
            _held.Remove(key);
            if (_parked.Contains(key))
            {
                // The board cannot have this one (see above): it goes back to full speed.
                _parkDue[key] = 0;
                if (TryPark(key)) _log("cooling pipe: " + key + " is at full speed until the PC restarts");
                return;
            }
            if (TryRelease(key)) _log("cooling pipe: " + key + " is back with the board");
            else if (_owed.Add(key)) _log("cooling pipe: " + key + " could not be given back yet, trying again");
        }

        private void RetryOwed()
        {
            foreach (string key in new List<string>(_owed))
            {
                if (_held.ContainsKey(key) || !TryRelease(key)) continue;
                _owed.Remove(key);
                _log("cooling pipe: " + key + " is back with the board");
            }
            foreach (string key in new List<string>(_parkDue.Keys))
            {
                if (_held.ContainsKey(key)) continue;
                if (TryPark(key))
                {
                    _log("cooling pipe: " + key + " is at full speed until the PC restarts");
                    continue;
                }
                if (++_parkDue[key] > ParkGiveUpSweeps)
                {
                    _log("cooling pipe: " + key + " cannot be reached, it is let go");
                    _parkDue.Remove(key);
                    _parked.Remove(key);
                }
            }
        }

        private bool TryRelease(string key)
        {
            try { return _ports.Release(key); }
            catch (Exception) { return false; }
        }

        private bool TryPark(string key)
        {
            bool done;
            try { done = _ports.Set(key, ParkPercent); }
            catch (Exception) { done = false; }
            if (done) _parkDue.Remove(key);
            return done;
        }

        private HoldRecord Build()
        {
            var rec = new HoldRecord();
            foreach (string key in _held.Keys) rec.Held.Add(key);
            foreach (string key in _owed) rec.Held.Add(key);
            foreach (string key in _parked)
            {
                rec.Held.Remove(key);
                rec.Parked.Add(key);
            }
            return rec;
        }

        private static string Describe(HoldRecord rec)
        {
            var held = new List<string>(rec.Held);
            var parked = new List<string>(rec.Parked);
            held.Sort(StringComparer.Ordinal);
            parked.Sort(StringComparer.Ordinal);
            return string.Join(",", held) + "|" + string.Join(",", parked);
        }

        /// <summary>
        /// Makes the journal tell what is held (and extra, which is about to be). False: it
        /// could not be written. A journal that is only behind on what was given back is
        /// written again by the next sweep.
        /// </summary>
        private bool Journal(string extra)
        {
            if (_journal == null) return true;
            HoldRecord rec = Build();
            if (extra != null && !rec.Parked.Contains(extra)) rec.Held.Add(extra);
            string now = Describe(rec);
            if (now == _saved)
            {
                _unsaved = false;
                return true;
            }
            if (_journal.Save(rec))
            {
                _saved = now;
                _unsaved = false;
                return true;
            }
            _unsaved = true;
            return false;
        }
    }
}
