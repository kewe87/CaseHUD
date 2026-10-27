// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using CaseHUD.Sensors.Smbus;

namespace CaseHUD.Sensors.Tests
{
    /// <summary>Records every transfer; can fail from the n-th transfer on.</summary>
    internal sealed class FakeBus : ISmbus
    {
        public readonly List<string> Calls = new List<string>();
        public bool ReadyResult = true;
        public int FailAt = -1; // index of the transfer that throws
        public byte ReadValue = 0x5A;
        public readonly Queue<byte> ReadQueue = new Queue<byte>();
        public Action Hook; // runs on every transfer

        // ENE emulation: commands 0xA0..0xAF read back 0..15, register 0x1000.. holds EneName
        public bool Ene;
        public string EneName = "";
        private ushort _reg;

        public bool Ready()
        {
            Calls.Add("ready");
            return ReadyResult;
        }

        public byte ReadByteData(byte addr, byte cmd)
        {
            Log("rbd " + addr.ToString("x2") + " " + cmd.ToString("x2"));
            if (Ene)
            {
                if (cmd >= 0xA0 && cmd <= 0xAF) return (byte)(cmd - 0xA0);
                if (cmd == 0x81)
                {
                    int i = _reg - 0x1000;
                    if (i >= 0 && i < EneName.Length) return (byte)EneName[i];
                    return 0;
                }
            }
            return ReadQueue.Count > 0 ? ReadQueue.Dequeue() : ReadValue;
        }

        public void WriteByteData(byte addr, byte cmd, byte value) => Log("wbd " + addr.ToString("x2") + " " + cmd.ToString("x2") + " " + value.ToString("x2"));

        public void WriteWordData(byte addr, byte cmd, ushort value)
        {
            Log("wwd " + addr.ToString("x2") + " " + cmd.ToString("x2") + " " + value.ToString("x4"));
            _reg = (ushort)(value << 8 | value >> 8);
        }

        public void WriteBlockData(byte addr, byte cmd, byte[] data) =>
            Log("wblk " + addr.ToString("x2") + " " + cmd.ToString("x2") + " " + BitConverter.ToString(data).Replace("-", "").ToLowerInvariant());

        private void Log(string s)
        {
            Calls.Add(s);
            Hook?.Invoke();
            if (FailAt >= 0 && Calls.Count - 1 >= FailAt + 1) throw new IOException("nack"); // +1: "ready" is call 0
        }
    }

    internal sealed class FakeLock : ISmbusLock
    {
        public bool Held; // another tool holds the mutex
        public int Acquired;
        public int Released;
        public int LastTimeout;

        public bool TryAcquire(int timeoutMs)
        {
            LastTimeout = timeoutMs;
            if (Held) return false;
            Acquired++;
            return true;
        }

        public void Release() => Released++;
    }

    /// <summary>Delivers the input a few bytes at a time and collects what is written.</summary>
    internal sealed class ScriptedStream : Stream
    {
        private readonly byte[] _input;
        private readonly int _chunk;
        private int _pos;
        public readonly MemoryStream Output = new MemoryStream();

        public ScriptedStream(byte[] input, int chunk = int.MaxValue)
        {
            _input = input;
            _chunk = chunk;
        }

        public override int Read(byte[] buffer, int offset, int count)
        {
            int n = Math.Min(Math.Min(count, _chunk), _input.Length - _pos);
            Buffer.BlockCopy(_input, _pos, buffer, offset, n);
            _pos += n;
            return n;
        }

        public override void Write(byte[] buffer, int offset, int count) => Output.Write(buffer, offset, count);
        public override void Flush() { }
        public override bool CanRead => true;
        public override bool CanWrite => true;
        public override bool CanSeek => false;
        public override long Length => throw new NotSupportedException();
        public override long Position { get => throw new NotSupportedException(); set => throw new NotSupportedException(); }
        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
        public override void SetLength(long value) => throw new NotSupportedException();

        /// <summary>Splits the written bytes into length-prefixed response bodies.</summary>
        public List<string> Responses()
        {
            var list = new List<string>();
            byte[] all = Output.ToArray();
            int i = 0;
            while (i + 4 <= all.Length)
            {
                int n = all[i] | all[i + 1] << 8 | all[i + 2] << 16 | all[i + 3] << 24;
                list.Add(Encoding.UTF8.GetString(all, i + 4, n));
                i += 4 + n;
            }
            return list;
        }
    }

    internal static class Frames
    {
        public static byte[] Frame(string json)
        {
            byte[] body = Encoding.UTF8.GetBytes(json);
            var msg = new byte[4 + body.Length];
            BitConverter.GetBytes((uint)body.Length).CopyTo(msg, 0);
            body.CopyTo(msg, 4);
            return msg;
        }

        public static byte[] Frames_(params string[] json)
        {
            var ms = new MemoryStream();
            foreach (string j in json) ms.Write(Frame(j), 0, Frame(j).Length);
            return ms.ToArray();
        }

        public static string Req(string ops, string addr = "113", string lockMs = "10", string v = "1") =>
            "{\"v\":" + v + ",\"addr\":" + addr + ",\"lockMs\":" + lockMs + ",\"ops\":[" + ops + "]}";
    }
}
