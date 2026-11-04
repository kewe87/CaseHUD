// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

namespace CaseHUD.Sensors.RingBoot
{
    /// <summary>
    /// The LED frame of the 8.8" ring: three 64-byte packets of 20 LEDs each.
    /// byte 0 is 0x11 (set LEDs), byte 1 the index of the first LED (0, 20, 40),
    /// bytes 2..3 are 0, bytes 4..63 hold 20 x R G B. This is the only command the
    /// service ever sends to the ring.
    /// </summary>
    internal static class RingFrame
    {
        public const byte CmdSetLeds = 0x11;
        public const int PacketLen = 64;
        public const int LedsPerPacket = 20;
        public const int Leds = 60;
        public const int Packets = Leds / LedsPerPacket;

        public static byte[][] Build(byte r, byte g, byte b)
        {
            var packets = new byte[Packets][];
            for (int p = 0; p < Packets; p++)
            {
                var pkt = new byte[PacketLen];
                pkt[0] = CmdSetLeds;
                pkt[1] = (byte)(p * LedsPerPacket);
                for (int i = 0; i < LedsPerPacket; i++)
                {
                    pkt[4 + 3 * i] = r;
                    pkt[5 + 3 * i] = g;
                    pkt[6 + 3 * i] = b;
                }
                packets[p] = pkt;
            }
            return packets;
        }

        /// <summary>Black for Off and None, the color otherwise.</summary>
        public static byte[][] Build(RingBootState s) =>
            s.Mode == RingBootMode.Color ? Build(s.R, s.G, s.B) : Build(0, 0, 0);

        /// <summary>The write guard: only complete 0x11 packets with a known LED offset go out.</summary>
        public static bool IsValidPacket(byte[] pkt)
        {
            if (pkt == null || pkt.Length != PacketLen || pkt[0] != CmdSetLeds) return false;
            return pkt[1] == 0 || pkt[1] == LedsPerPacket || pkt[1] == 2 * LedsPerPacket;
        }
    }
}
