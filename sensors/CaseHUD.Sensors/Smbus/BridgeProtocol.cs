// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text;
using System.Web.Script.Serialization;

namespace CaseHUD.Sensors.Smbus
{
    internal enum OpKind
    {
        ReadReg,   // "rb"
        WriteReg,  // "wb"
        Block,     // "blk"
        ReadCmd,   // "rcmd"
    }

    internal sealed class BridgeOp
    {
        public OpKind Kind;
        public ushort Reg;
        public byte Val;
        public byte[] Data;
        public byte Cmd;
    }

    internal sealed class BridgeRequest
    {
        public byte Addr;
        public int LockMs;
        public List<BridgeOp> Ops = new List<BridgeOp>();

        /// <summary>True if any op goes beyond the read-only identification commands.</summary>
        public bool NeedsVerifiedDevice
        {
            get
            {
                foreach (BridgeOp o in Ops) if (o.Kind != OpKind.ReadCmd) return true;
                return false;
            }
        }
    }

    /// <summary>The ENE registers the bridge lets a client touch (direct lighting only).</summary>
    internal static class EneRegisters
    {
        public const int DeviceName = 0x1000;     // 16 chars, read only
        public const int DirectFirst = 0x8020;    // direct / mode / speed / direction
        public const int DirectLast = 0x8023;
        public const int ApplyLegacy = 0x802F;
        public const int Apply = 0x80A0;          // value 0xAA there means "save to EEPROM"

        public static readonly HashSet<string> KnownDevices = new HashSet<string>
        {
            "LED-0116", "AUMA0-E8K4-0101", "DIMM_LED-0102", "DIMM_LED-0103",
            "AUDA0-E6K5-0101", "AUMA0-E6K5-0104", "AUMA0-E6K5-0105", "AUMA0-E6K5-0106",
        };

        /// <summary>Single-byte writes: direct/mode registers, and apply with the value 1 only.</summary>
        public static bool WriteAllowed(int reg, int val)
        {
            if (reg >= DirectFirst && reg <= DirectLast) return true;
            if (reg == Apply || reg == ApplyLegacy) return val == 0x01;
            return false;
        }

        /// <summary>Block writes: the color tables (v1 direct/effect, v2 direct, v2 effect) as a whole.</summary>
        public static bool BlockAllowed(int reg, int len)
        {
            int end = reg + len;
            return InRange(reg, end, 0x8000, 0x8020)
                || InRange(reg, end, 0x8100, 0x8140)
                || InRange(reg, end, 0x8160, 0x81A0);
        }

        private static bool InRange(int start, int end, int lo, int hi) => start >= lo && end <= hi;
    }

    /// <summary>
    /// Wire format of the SMBus bridge: uint32 little-endian length followed by UTF-8
    /// JSON, and the strict validation of requests. Nothing the client sends is trusted.
    /// </summary>
    internal static class BridgeProtocol
    {
        public const int MaxMessage = 64 << 10;
        public const int MaxOps = 64;
        public const int MaxBlock = 32;
        public const int MaxLockMs = 50;
        public const int AddrMin = 0x70;
        public const int AddrMax = 0x77;
        public const int CmdMin = 0xA0;
        public const int CmdMax = 0xAF;

        /// <summary>Returns the request, or null with a reason for the (rate-limited) log.</summary>
        public static BridgeRequest Parse(byte[] body, out string reason)
        {
            reason = null;
            object root;
            try
            {
                string json = new UTF8Encoding(false, true).GetString(body);
                root = new JavaScriptSerializer { MaxJsonLength = MaxMessage * 2, RecursionLimit = 8 }.DeserializeObject(json);
            }
            catch (Exception)
            {
                reason = "malformed JSON";
                return null;
            }
            var obj = root as Dictionary<string, object>;
            if (obj == null)
            {
                reason = "not an object";
                return null;
            }

            long v, addr, lockMs;
            if (!TryInt(obj, "v", 0, 1000000, out v) || v != 1) { reason = "unsupported version"; return null; }
            if (!TryInt(obj, "addr", 0, 255, out addr) || addr < AddrMin || addr > AddrMax) { reason = "address not allowed"; return null; }
            if (!TryInt(obj, "lockMs", 0, 1000000, out lockMs) || lockMs < 1 || lockMs > MaxLockMs) { reason = "lockMs out of range"; return null; }

            object opsObj;
            var ops = obj.TryGetValue("ops", out opsObj) ? opsObj as System.Collections.IList : null;
            if (ops == null || ops.Count < 1 || ops.Count > MaxOps) { reason = "op count out of range"; return null; }

            var req = new BridgeRequest { Addr = (byte)addr, LockMs = (int)lockMs };
            foreach (object item in ops)
            {
                var o = item as Dictionary<string, object>;
                if (o == null) { reason = "op is not an object"; return null; }
                object kindObj;
                string kind = o.TryGetValue("op", out kindObj) ? kindObj as string : null;
                var op = new BridgeOp();
                long n;
                switch (kind)
                {
                    case "rb":
                    case "wb":
                    case "blk":
                        op.Kind = kind == "rb" ? OpKind.ReadReg : kind == "wb" ? OpKind.WriteReg : OpKind.Block;
                        if (!TryInt(o, "reg", 0, 0xFFFF, out n)) { reason = "register out of range"; return null; }
                        op.Reg = (ushort)n;
                        if (op.Kind == OpKind.WriteReg)
                        {
                            if (!TryInt(o, "val", 0, 255, out n)) { reason = "value out of range"; return null; }
                            op.Val = (byte)n;
                            if (!EneRegisters.WriteAllowed(op.Reg, op.Val)) { reason = "register write not allowed"; return null; }
                        }
                        else if (op.Kind == OpKind.Block)
                        {
                            object dataObj;
                            string b64 = o.TryGetValue("data", out dataObj) ? dataObj as string : null;
                            // 32 bytes are at most 44 base64 characters; check before decoding.
                            if (b64 == null || b64.Length == 0 || b64.Length > 4 * ((MaxBlock + 2) / 3)) { reason = "block size not allowed"; return null; }
                            byte[] data;
                            try { data = Convert.FromBase64String(b64); }
                            catch (FormatException) { reason = "block is not base64"; return null; }
                            if (data.Length < 1 || data.Length > MaxBlock) { reason = "block size not allowed"; return null; }
                            op.Data = data;
                            if (!EneRegisters.BlockAllowed(op.Reg, data.Length)) { reason = "block write not allowed"; return null; }
                        }
                        break;
                    case "rcmd":
                        op.Kind = OpKind.ReadCmd;
                        if (!TryInt(o, "cmd", 0, 255, out n) || n < CmdMin || n > CmdMax) { reason = "raw command not allowed"; return null; }
                        op.Cmd = (byte)n;
                        break;
                    default:
                        reason = "unknown op";
                        return null;
                }
                req.Ops.Add(op);
            }
            return req;
        }

        // A missing field counts as 0 (the Go client omits zero values).
        private static bool TryInt(Dictionary<string, object> obj, string key, long min, long max, out long value)
        {
            value = 0;
            object raw;
            if (!obj.TryGetValue(key, out raw)) return true;
            if (raw is int) value = (int)raw;
            else if (raw is long) value = (long)raw;
            else if (raw is decimal && decimal.Truncate((decimal)raw) == (decimal)raw && (decimal)raw >= min && (decimal)raw <= max) value = (long)(decimal)raw;
            else if (raw is double && Math.Floor((double)raw) == (double)raw && (double)raw >= min && (double)raw <= max) value = (long)(double)raw;
            else return false;
            return value >= min && value <= max;
        }

        public static byte[] Ok(IList<byte> reads)
        {
            var sb = new StringBuilder("{\"ok\":true,\"reads\":[");
            for (int i = 0; i < reads.Count; i++)
            {
                if (i > 0) sb.Append(',');
                sb.Append(reads[i].ToString(CultureInfo.InvariantCulture));
            }
            return Encoding.UTF8.GetBytes(sb.Append("]}").ToString());
        }

        /// <summary>error is "timeout", "denied" or "io".</summary>
        public static byte[] Error(string error) => Encoding.UTF8.GetBytes("{\"ok\":false,\"error\":\"" + error + "\"}");

        public enum ReadResult
        {
            Ok,
            Closed,   // peer went away (also in the middle of a message)
            Invalid,  // length zero or above the limit: the connection must be dropped
        }

        /// <summary>Reads one length-prefixed message; copes with partial reads.</summary>
        public static ReadResult ReadMessage(Stream s, out byte[] body)
        {
            body = null;
            var hdr = new byte[4];
            if (!ReadFull(s, hdr, 4)) return ReadResult.Closed;
            uint n = (uint)(hdr[0] | hdr[1] << 8 | hdr[2] << 16 | hdr[3] << 24);
            if (n == 0 || n > MaxMessage) return ReadResult.Invalid;
            var buf = new byte[n];
            if (!ReadFull(s, buf, (int)n)) return ReadResult.Closed;
            body = buf;
            return ReadResult.Ok;
        }

        public static void WriteMessage(Stream s, byte[] body)
        {
            var msg = new byte[4 + body.Length];
            msg[0] = (byte)body.Length;
            msg[1] = (byte)(body.Length >> 8);
            msg[2] = (byte)(body.Length >> 16);
            msg[3] = (byte)(body.Length >> 24);
            Buffer.BlockCopy(body, 0, msg, 4, body.Length);
            s.Write(msg, 0, msg.Length);
            s.Flush();
        }

        private static bool ReadFull(Stream s, byte[] buf, int count)
        {
            int got = 0;
            while (got < count)
            {
                int r = s.Read(buf, got, count - got);
                if (r <= 0) return false;
                got += r;
            }
            return true;
        }
    }
}
