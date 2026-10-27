// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Text;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Runs validated batches on the bus. The whole batch is one acquisition of the
    /// shared SMBus mutex, waited for at most lockMs. ENE access follows the
    /// controller's indirect scheme: the register address goes to command 0x00 as a
    /// byte-swapped word, then data moves through 0x81 (read), 0x01 (write) or
    /// 0x03 (block write).
    /// </summary>
    internal sealed class BridgeExecutor
    {
        public const byte CmdRegister = 0x00;
        public const byte CmdWrite = 0x01;
        public const byte CmdBlock = 0x03;
        public const byte CmdRead = 0x81;

        private readonly ISmbus _bus;
        private readonly ISmbusLock _lock;
        private readonly Action<string> _log;
        private readonly HoldBudget _budget;
        private bool _lockErrorLogged;
        private readonly HashSet<byte> _notEneLogged = new HashSet<byte>();

        /// <summary>Addresses proven to be ENE DRAM controllers; only those may be written.</summary>
        internal EneVerifier Verifier { get; } = new EneVerifier();

        public BridgeExecutor(ISmbus bus, ISmbusLock busLock, Action<string> log, HoldBudget budget = null)
        {
            _bus = bus;
            _lock = busLock;
            _log = log;
            _budget = budget;
        }

        /// <summary>Returns the response body for a request.</summary>
        public byte[] Execute(BridgeRequest req)
        {
            if (_budget != null && !_budget.Allow()) return BridgeProtocol.Error("timeout");
            try
            {
                if (!_lock.TryAcquire(req.LockMs)) return BridgeProtocol.Error("timeout");
            }
            catch (Exception e)
            {
                // e.g. the ACL of the mutex does not let this account open it
                if (!_lockErrorLogged) _log("SMBus bridge: cannot use the SMBus mutex: " + e.Message);
                _lockErrorLogged = true;
                return BridgeProtocol.Error("timeout");
            }
            long held = _budget != null ? _budget.NowUs() : 0;
            try
            {
                if (!_bus.Ready()) return BridgeProtocol.Error("timeout");
                // Anything but the read-only identification needs a verified ENE controller:
                // other devices live in this address range (e.g. PCA954x muxes).
                if (req.NeedsVerifiedDevice && !Verifier.IsVerified(req.Addr))
                {
                    if (!VerifyEne(req.Addr))
                    {
                        if (_notEneLogged.Add(req.Addr))
                            _log("SMBus bridge: no ENE DRAM controller at 0x" + req.Addr.ToString("x2") + ", access denied");
                        return BridgeProtocol.Error("denied");
                    }
                    _notEneLogged.Remove(req.Addr);
                    Verifier.Mark(req.Addr);
                }
                var reads = new List<byte>();
                foreach (BridgeOp op in req.Ops) Run(req.Addr, op, reads);
                return BridgeProtocol.Ok(reads);
            }
            catch (Exception e)
            {
                Verifier.Clear(req.Addr); // re-verify before the next write
                _log("SMBus transfer failed: " + e.Message);
                return BridgeProtocol.Error("io");
            }
            finally
            {
                if (_budget != null) _budget.Record(_budget.NowUs() - held);
                try { _lock.Release(); } catch { }
            }
        }

        /// <summary>
        /// Same identification as ENE detection: the controller answers commands 0xA0..0xAF
        /// with 0x00..0x0F (read only, so it is safe on any device), and register 0x1000
        /// holds a known device name. Throws on transfer errors.
        /// </summary>
        private bool VerifyEne(byte addr)
        {
            int pass = 0;
            for (int i = 0; i < 16; i++)
                if (_bus.ReadByteData(addr, (byte)(BridgeProtocol.CmdMin + i)) == i) pass++;
            if (pass < 15) return false;
            var name = new StringBuilder();
            for (int i = 0; i < 16; i++)
            {
                SetRegister(addr, (ushort)(EneRegisters.DeviceName + i));
                byte c = _bus.ReadByteData(addr, CmdRead);
                if (c > 0) name.Append((char)c);
            }
            return EneRegisters.KnownDevices.Contains(name.ToString());
        }

        private void Run(byte addr, BridgeOp op, List<byte> reads)
        {
            switch (op.Kind)
            {
                case OpKind.ReadCmd:
                    reads.Add(_bus.ReadByteData(addr, op.Cmd));
                    break;
                case OpKind.ReadReg:
                    SetRegister(addr, op.Reg);
                    reads.Add(_bus.ReadByteData(addr, CmdRead));
                    break;
                case OpKind.WriteReg:
                    SetRegister(addr, op.Reg);
                    _bus.WriteByteData(addr, CmdWrite, op.Val);
                    break;
                case OpKind.Block:
                    SetRegister(addr, op.Reg);
                    _bus.WriteBlockData(addr, CmdBlock, op.Data);
                    break;
            }
        }

        private void SetRegister(byte addr, ushort reg) => _bus.WriteWordData(addr, CmdRegister, (ushort)(reg << 8 | reg >> 8));
    }

    /// <summary>Remembers which addresses passed the ENE identification.</summary>
    internal sealed class EneVerifier
    {
        private readonly HashSet<byte> _ok = new HashSet<byte>();

        public bool IsVerified(byte addr) { lock (_ok) return _ok.Contains(addr); }
        public void Mark(byte addr) { lock (_ok) _ok.Add(addr); }
        public void Clear(byte addr) { lock (_ok) _ok.Remove(addr); }
    }

    /// <summary>Token bucket: at most <c>rate</c> events per second, with a burst of the same size.</summary>
    internal sealed class RateLimiter
    {
        private readonly double _rate;
        private readonly Func<long> _nowMs;
        private double _tokens;
        private long _last;

        public RateLimiter(double perSecond, Func<long> nowMs = null)
        {
            _rate = perSecond;
            _nowMs = nowMs ?? (() => Stopwatch.GetTimestamp() * 1000 / Stopwatch.Frequency);
            _tokens = perSecond;
            _last = _nowMs();
        }

        public bool Allow()
        {
            lock (this) return AllowLocked();
        }

        private bool AllowLocked()
        {
            long now = _nowMs();
            _tokens = Math.Min(_rate, _tokens + (now - _last) * _rate / 1000.0);
            _last = now;
            if (_tokens < 1) return false;
            _tokens -= 1;
            return true;
        }
    }

    /// <summary>
    /// Caps how long the SMBus mutex is held over all connections: at most half of the
    /// time in a one-second window, so the bridge cannot starve other SMBus users.
    /// </summary>
    internal sealed class HoldBudget
    {
        public const long WindowUs = 1000000;
        public const long MaxHeldUs = WindowUs / 2;

        private readonly Func<long> _nowUs;
        private readonly object _sync = new object();
        private long _windowStart;
        private long _heldUs;

        public HoldBudget(Func<long> nowUs = null)
        {
            _nowUs = nowUs ?? (() => Stopwatch.GetTimestamp() * 1000000 / Stopwatch.Frequency);
            _windowStart = _nowUs();
        }

        public long NowUs() => _nowUs();

        public bool Allow()
        {
            lock (_sync)
            {
                Roll();
                return _heldUs < MaxHeldUs;
            }
        }

        public void Record(long heldUs)
        {
            lock (_sync)
            {
                Roll();
                _heldUs += Math.Max(0, heldUs);
            }
        }

        private void Roll()
        {
            long now = _nowUs();
            if (now - _windowStart >= WindowUs)
            {
                _windowStart = now;
                _heldUs = 0;
            }
        }
    }

    /// <summary>Logs refused requests, at most one line per interval so a misbehaving client cannot flood the log.</summary>
    internal sealed class DenyLog
    {
        private readonly Action<string> _log;
        private readonly Func<long> _nowMs;
        private readonly long _intervalMs;
        private readonly object _sync = new object();
        private long _next = long.MinValue;
        private int _suppressed;

        public DenyLog(Action<string> log, Func<long> nowMs = null, long intervalMs = 10000)
        {
            _log = log;
            _nowMs = nowMs ?? (() => Stopwatch.GetTimestamp() * 1000 / Stopwatch.Frequency);
            _intervalMs = intervalMs;
        }

        public void Denied(string reason)
        {
            lock (_sync)
            {
                long now = _nowMs();
                if (now < _next)
                {
                    _suppressed++;
                    return;
                }
                _next = now + _intervalMs;
                string more = _suppressed > 0 ? " (+" + _suppressed + " more)" : "";
                _suppressed = 0;
                _log("SMBus bridge denied a request: " + reason + more);
            }
        }
    }
}
