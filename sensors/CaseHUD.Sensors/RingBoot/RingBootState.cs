// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;
using System.Web.Script.Serialization;

namespace CaseHUD.Sensors.RingBoot
{
    internal enum RingBootMode
    {
        None,   // leave the ring alone (the firmware rainbow keeps running)
        Off,    // all LEDs black
        Color,  // one static color
    }

    /// <summary>
    /// What the ring should show right after Windows starts. Immutable; the only way
    /// to build one from outside data is <see cref="TryFromDictionary"/> / <see cref="ParseFile"/>,
    /// which are strict: anything unexpected is rejected.
    /// </summary>
    internal sealed class RingBootState : IEquatable<RingBootState>
    {
        public const int MaxJsonBytes = 1024;

        public static readonly RingBootState None = new RingBootState(RingBootMode.None, 0, 0, 0);
        public static readonly RingBootState Off = new RingBootState(RingBootMode.Off, 0, 0, 0);

        public RingBootMode Mode { get; }
        public byte R { get; }
        public byte G { get; }
        public byte B { get; }

        private RingBootState(RingBootMode mode, byte r, byte g, byte b)
        {
            Mode = mode;
            R = r;
            G = g;
            B = b;
        }

        public static RingBootState FromColor(byte r, byte g, byte b) => new RingBootState(RingBootMode.Color, r, g, b);

        public bool Equals(RingBootState o) => o != null && Mode == o.Mode && R == o.R && G == o.G && B == o.B;
        public override bool Equals(object obj) => Equals(obj as RingBootState);
        public override int GetHashCode() => (int)Mode << 24 ^ R << 16 ^ G << 8 ^ B;

        public static string ModeName(RingBootMode m) => m == RingBootMode.Off ? "off" : m == RingBootMode.Color ? "color" : "none";

        public override string ToString() =>
            Mode == RingBootMode.Color ? "color #" + R.ToString("x2") + G.ToString("x2") + B.ToString("x2") : ModeName(Mode);

        /// <summary>The file content; off and none always carry zeros.</summary>
        public string ToJson() =>
            "{\"mode\":\"" + ModeName(Mode) + "\",\"r\":" + R.ToString(CultureInfo.InvariantCulture) +
            ",\"g\":" + G.ToString(CultureInfo.InvariantCulture) + ",\"b\":" + B.ToString(CultureInfo.InvariantCulture) + "}";

        /// <summary>Parses the file content. Anything invalid (including oversized or malformed input) yields None.</summary>
        public static RingBootState ParseFile(byte[] content)
        {
            RingBootState s;
            return TryParseFile(content, out s) ? s : None;
        }

        /// <summary>Strict parse of the file content; false if it is not a valid state.</summary>
        public static bool TryParseFile(byte[] content, out RingBootState state)
        {
            state = None;
            if (content == null || content.Length == 0 || content.Length > MaxJsonBytes) return false;
            try
            {
                string json = new UTF8Encoding(false, true).GetString(content);
                object root = new JavaScriptSerializer { MaxJsonLength = MaxJsonBytes * 2, RecursionLimit = 4 }.DeserializeObject(json);
                string reason;
                return TryFromDictionary(root as Dictionary<string, object>, null, out state, out reason);
            }
            catch (Exception)
            {
                state = None;
                return false;
            }
        }

        /// <summary>
        /// Strict validation shared by the file and the pipe request. Allowed keys are
        /// "mode", "r", "g", "b" plus the names in <paramref name="extraKeys"/> (envelope
        /// fields of the pipe message). "mode" must be a known string; r/g/b must be
        /// integers 0..255 (booleans, strings and fractions are refused) and are required
        /// for "color". For "off" and "none" they may be absent or 0..255 but are dropped.
        /// </summary>
        public static bool TryFromDictionary(Dictionary<string, object> obj, ICollection<string> extraKeys, out RingBootState state, out string reason)
        {
            state = None;
            reason = null;
            if (obj == null) { reason = "not an object"; return false; }
            foreach (string k in obj.Keys)
            {
                if (k == "mode" || k == "r" || k == "g" || k == "b") continue;
                if (extraKeys != null && extraKeys.Contains(k)) continue;
                reason = "unknown field";
                return false;
            }
            object m;
            string mode = obj.TryGetValue("mode", out m) ? m as string : null;
            if (mode != "off" && mode != "color" && mode != "none") { reason = "mode not allowed"; return false; }

            int r, g, b;
            bool hasR = Channel(obj, "r", out r), hasG = Channel(obj, "g", out g), hasB = Channel(obj, "b", out b);
            if ((hasR && r < 0) || (hasG && g < 0) || (hasB && b < 0)) { reason = "color channel out of range"; return false; }
            if (mode == "color")
            {
                if (!hasR || !hasG || !hasB) { reason = "color needs r, g and b"; return false; }
                state = FromColor((byte)r, (byte)g, (byte)b);
            }
            else
            {
                state = mode == "off" ? Off : None;
            }
            return true;
        }

        // Returns whether the key is present; the value is -1 if it is present but not an integer 0..255.
        private static bool Channel(Dictionary<string, object> obj, string key, out int value)
        {
            value = -1;
            object raw;
            if (!obj.TryGetValue(key, out raw)) { value = 0; return false; }
            long n;
            if (raw is int) n = (int)raw;
            else if (raw is long) n = (long)raw;
            else return true; // bool, string, decimal/double (fractions, exponents), null, nested: invalid
            if (n >= 0 && n <= 255) value = (int)n;
            return true;
        }
    }
}
