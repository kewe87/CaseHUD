// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;

namespace CaseHUD.Sensors.Http
{
    /// <summary>
    /// The sensor service answers requests addressed to itself only. A web page that
    /// reaches 127.0.0.1 through a rebound DNS name sends that name as Host.
    /// </summary>
    internal static class HostCheck
    {
        public static bool Allowed(string host, int port)
        {
            if (string.IsNullOrEmpty(host)) return false;
            string suffix = ":" + port.ToString(System.Globalization.CultureInfo.InvariantCulture);
            if (!host.EndsWith(suffix, StringComparison.Ordinal)) return false;
            string name = host.Substring(0, host.Length - suffix.Length);
            return name == "127.0.0.1" || string.Equals(name, "localhost", StringComparison.OrdinalIgnoreCase);
        }
    }
}
