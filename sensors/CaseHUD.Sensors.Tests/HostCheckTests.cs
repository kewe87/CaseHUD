// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using CaseHUD.Sensors.Http;
using Xunit;

namespace CaseHUD.Sensors.Tests
{
    public class HostCheckTests
    {
        [Theory]
        [InlineData("127.0.0.1:8086")]
        [InlineData("localhost:8086")]
        [InlineData("LocalHost:8086")]
        public void AddressedToTheServiceIsAllowed(string host) => Assert.True(HostCheck.Allowed(host, 8086));

        [Theory]
        [InlineData(null)]
        [InlineData("")]
        [InlineData("evil.example")]
        [InlineData("evil.example:8086")]
        [InlineData("127.0.0.1")]
        [InlineData("127.0.0.1:8085")]
        [InlineData("127.0.0.1:80860")]
        [InlineData("127.0.0.1.evil.example:8086")]
        [InlineData("x127.0.0.1:8086")]
        [InlineData("localhost.evil.example:8086")]
        [InlineData("[::1]:8086")]
        [InlineData("user@127.0.0.1:8086")]
        public void EverythingElseIsRefused(string host) => Assert.False(HostCheck.Allowed(host, 8086));
    }
}
