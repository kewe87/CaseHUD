// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Bookkeeping for the open client connections: a cap on the total and per client
    /// process, an idle timeout, and an orderly shutdown that cannot race with new
    /// connections (once closed, nothing is added any more).
    /// </summary>
    internal sealed class ConnectionTracker
    {
        private sealed class Entry
        {
            public Stream Raw;
            public int Pid;
            public long LastActive;
        }

        private readonly object _sync = new object();
        private readonly List<Entry> _entries = new List<Entry>();
        private readonly Func<long> _nowMs;
        private readonly int _maxTotal;
        private readonly int _maxPerProcess;
        private bool _closed;

        public ConnectionTracker(int maxTotal, int maxPerProcess, Func<long> nowMs = null)
        {
            _maxTotal = maxTotal;
            _maxPerProcess = maxPerProcess;
            _nowMs = nowMs ?? (() => System.Diagnostics.Stopwatch.GetTimestamp() * 1000 / System.Diagnostics.Stopwatch.Frequency);
        }

        public int Count
        {
            get { lock (_sync) return _entries.Count; }
        }

        /// <summary>
        /// Registers a connection and returns the stream to serve it on (disposing it
        /// unregisters), or null if the tracker is closed or a limit is reached; the
        /// caller then drops the connection. pid 0 means unknown and is only counted in the total.
        /// </summary>
        public Stream TryAdd(Stream raw, int pid)
        {
            lock (_sync)
            {
                if (_closed || _entries.Count >= _maxTotal) return null;
                if (pid != 0)
                {
                    int same = 0;
                    foreach (Entry x in _entries) if (x.Pid == pid) same++;
                    if (same >= _maxPerProcess) return null;
                }
                var e = new Entry { Raw = raw, Pid = pid, LastActive = _nowMs() };
                _entries.Add(e);
                return new ActivityStream(this, e);
            }
        }

        /// <summary>Closes connections that did not send or receive anything for idleMs.</summary>
        public int CloseIdle(long idleMs)
        {
            var idle = new List<Entry>();
            lock (_sync)
            {
                long now = _nowMs();
                foreach (Entry e in _entries) if (now - e.LastActive >= idleMs) idle.Add(e);
            }
            foreach (Entry e in idle) Dispose(e.Raw);
            return idle.Count;
        }

        /// <summary>Closes every connection and refuses new ones.</summary>
        public void CloseAll()
        {
            Entry[] all;
            lock (_sync)
            {
                _closed = true;
                all = _entries.ToArray();
            }
            foreach (Entry e in all) Dispose(e.Raw);
        }

        private void Remove(Entry e)
        {
            lock (_sync) _entries.Remove(e);
        }

        private static void Dispose(Stream s)
        {
            try { s.Dispose(); } catch { }
        }

        private sealed class ActivityStream : Stream
        {
            private readonly ConnectionTracker _owner;
            private readonly Entry _entry;

            public ActivityStream(ConnectionTracker owner, Entry entry)
            {
                _owner = owner;
                _entry = entry;
            }

            private void Touch() => _entry.LastActive = _owner._nowMs();

            public override int Read(byte[] buffer, int offset, int count)
            {
                int n = _entry.Raw.Read(buffer, offset, count);
                Touch();
                return n;
            }

            public override void Write(byte[] buffer, int offset, int count)
            {
                _entry.Raw.Write(buffer, offset, count);
                Touch();
            }

            public override void Flush() => _entry.Raw.Flush();

            protected override void Dispose(bool disposing)
            {
                if (disposing)
                {
                    _owner.Remove(_entry);
                    ConnectionTracker.Dispose(_entry.Raw);
                }
                base.Dispose(disposing);
            }

            public override bool CanRead => true;
            public override bool CanWrite => true;
            public override bool CanSeek => false;
            public override long Length => throw new NotSupportedException();
            public override long Position { get => throw new NotSupportedException(); set => throw new NotSupportedException(); }
            public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
            public override void SetLength(long value) => throw new NotSupportedException();
        }
    }
}
