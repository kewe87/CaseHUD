// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Threading;
using Microsoft.Win32.SafeHandles;

namespace CaseHUD.Sensors.Smbus
{
    /// <summary>
    /// Owns the bridge pipe: it exists only while the "smbusBridge" setting is true.
    /// The setting is re-read periodically, so enabling or disabling needs no restart.
    /// </summary>
    internal sealed class SmbusBridgeHost : IDisposable
    {
        public const string PipeName = "CaseHUD.Smbus";
        public const int PollMs = 10000;

        private readonly Func<bool> _enabled;
        private readonly Func<IPipeServer> _create;
        private readonly Action<string> _log;
        private readonly object _sync = new object();
        private IPipeServer _server;
        private Timer _timer;
        private bool _failureLogged;
        private bool _disposed;

        public SmbusBridgeHost(Func<bool> enabled, Func<IPipeServer> create, Action<string> log)
        {
            _enabled = enabled;
            _create = create;
            _log = log;
        }

        /// <summary>The wiring used by the service: the real config file, PawnIO and a secured named pipe.</summary>
        public static SmbusBridgeHost CreateDefault(Func<Stream> module, Action<string> log)
        {
            var budget = new HoldBudget();
            var bus = new LazySmbus(() => PawnIoSmbus.Open(module), log);
            var executor = new BridgeExecutor(bus, new SystemSmbusLock(), log, budget);
            var denyLog = new DenyLog(log);
            var shared = new RateLimiter(BridgeConnection.TotalBatchesPerSecond);
            var configLog = new ChangeLog(log);
            return new SmbusBridgeHost(
                () => configLog.Track(() => BridgeConfig.ReadSmbusBridge(BridgeConfig.DefaultPath, configLog.Write)),
                () => new NamedPipeBridgeServer(PipeName, s => new BridgeConnection(executor, denyLog, null, shared).Serve(s), log),
                log);
        }

        public void Start()
        {
            Poll();
            lock (_sync)
            {
                if (!_disposed) _timer = new Timer(_ => Poll(), null, PollMs, PollMs);
            }
        }

        /// <summary>Applies the current setting: opens or closes the pipe, and restarts one whose accept loop died.</summary>
        public void Poll()
        {
            lock (_sync)
            {
                if (_disposed) return;
                bool want;
                try { want = _enabled(); }
                catch (Exception e) { _log("SMBus bridge: reading the setting failed: " + e.Message); want = false; }

                if (_server != null && want && !_server.IsAlive)
                {
                    _log("SMBus bridge: pipe stopped unexpectedly, restarting");
                    try { _server.Dispose(); } catch { }
                    _server = null;
                }

                if (want && _server == null)
                {
                    try
                    {
                        IPipeServer server = _create();
                        server.Start();
                        _server = server;
                        _failureLogged = false;
                        _log("SMBus bridge enabled: pipe \\\\.\\pipe\\" + PipeName);
                    }
                    catch (Exception e)
                    {
                        // e.g. the pipe name is already taken by another process: stay off, retry next poll
                        if (!_failureLogged) _log("SMBus bridge: starting the pipe failed (staying off, retrying): " + e.Message);
                        _failureLogged = true;
                    }
                }
                else if (!want && _server != null)
                {
                    try { _server.Dispose(); } catch { }
                    _server = null;
                    _log("SMBus bridge disabled");
                }
            }
        }

        public bool IsRunning
        {
            get { lock (_sync) return _server != null; }
        }

        public void Dispose()
        {
            lock (_sync)
            {
                if (_disposed) return;
                _disposed = true;
                _timer?.Dispose();
                if (_server != null)
                {
                    try { _server.Dispose(); } catch { }
                    _server = null;
                }
            }
        }
    }

    /// <summary>Passes a message on only if it differs from the previous one, so a polled check does not repeat itself.</summary>
    internal sealed class ChangeLog
    {
        private readonly Action<string> _log;
        private string _last;
        private int _written;

        public ChangeLog(Action<string> log) => _log = log;

        public void Write(string message)
        {
            _written++;
            if (message == _last) return;
            _last = message;
            _log(message);
        }

        /// <summary>Runs a check; if it wrote nothing, the next message counts as new again.</summary>
        public T Track<T>(Func<T> check)
        {
            int before = _written;
            T result = check();
            if (_written == before) _last = null;
            return result;
        }
    }

    /// <summary>
    /// Named pipe server for interactive users and administrators. Network logons and
    /// anonymous clients are denied. One thread per connection; the number of
    /// connections is capped in total and per client process, and idle ones are closed.
    /// </summary>
    internal sealed class NamedPipeBridgeServer : IPipeServer
    {
        public const int MaxConnections = 8;
        public const int MaxPerProcess = 2;
        public const long IdleMs = 60000;
        private const int SweepMs = 5000;
        private const int MaxInstances = 254;

        private readonly string _name;
        private readonly Action<Stream> _serve;
        private readonly Action<string> _log;
        private readonly long _idleMs;
        private readonly int _sweepMs;
        private readonly ConnectionTracker _tracker;
        private readonly ManualResetEvent _stop = new ManualResetEvent(false);
        private Thread _accept;
        private Timer _sweepTimer;
        private volatile bool _alive;
        private int _errors;

        public NamedPipeBridgeServer(string name, Action<Stream> serve, Action<string> log,
            int maxConnections = MaxConnections, int maxPerProcess = MaxPerProcess, long idleMs = IdleMs, int sweepMs = SweepMs)
        {
            _name = name;
            _serve = serve;
            _log = log;
            _idleMs = idleMs;
            _sweepMs = sweepMs;
            _tracker = new ConnectionTracker(maxConnections, maxPerProcess);
        }

        public bool IsAlive => _alive;

        public static PipeSecurity CreateSecurity()
        {
            var sec = new PipeSecurity();
            var network = new SecurityIdentifier(WellKnownSidType.NetworkSid, null);
            var anonymous = new SecurityIdentifier(WellKnownSidType.AnonymousSid, null);
            var interactive = new SecurityIdentifier(WellKnownSidType.InteractiveSid, null);
            var admins = new SecurityIdentifier(WellKnownSidType.BuiltinAdministratorsSid, null);
            var system = new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null);
            sec.AddAccessRule(new PipeAccessRule(network, PipeAccessRights.FullControl, AccessControlType.Deny));
            sec.AddAccessRule(new PipeAccessRule(anonymous, PipeAccessRights.FullControl, AccessControlType.Deny));
            sec.AddAccessRule(new PipeAccessRule(system, PipeAccessRights.FullControl, AccessControlType.Allow));
            sec.AddAccessRule(new PipeAccessRule(admins, PipeAccessRights.FullControl, AccessControlType.Allow));
            sec.AddAccessRule(new PipeAccessRule(interactive, PipeAccessRights.ReadWrite | PipeAccessRights.Synchronize, AccessControlType.Allow));
            // The account running the server must be able to create further instances
            // (always true for SYSTEM; matters when run as a normal user, e.g. in tests).
            SecurityIdentifier me = WindowsIdentity.GetCurrent().User;
            if (me != null && me != system && me != admins)
            {
                sec.AddAccessRule(new PipeAccessRule(me, PipeAccessRights.ReadWrite | PipeAccessRights.CreateNewInstance | PipeAccessRights.Synchronize, AccessControlType.Allow));
            }
            return sec;
        }

        /// <summary>
        /// Creates the first pipe instance with FILE_FLAG_FIRST_PIPE_INSTANCE, so it fails if
        /// another process already owns the name (squatting); the exception reaches the host.
        /// </summary>
        public void Start()
        {
            NamedPipeServerStream first = NewInstance(true);
            _alive = true;
            _sweepTimer = new Timer(_ => Sweep(), null, _sweepMs, _sweepMs);
            _accept = new Thread(() => AcceptLoop(first)) { IsBackground = true, Name = "smbus-bridge-accept" };
            _accept.Start();
        }

        // The instance limit is high on purpose: the real limit is the tracker. A limit equal
        // to the connection cap would make creating the next waiting instance fail.
        private NamedPipeServerStream NewInstance(bool first) => CreateInstance(_name, first);

        // .NET does not accept FILE_FLAG_FIRST_PIPE_INSTANCE in PipeOptions, and does not
        // promise PIPE_REJECT_REMOTE_CLIENTS, so every instance is created with
        // CreateNamedPipe and wrapped.
        private static NamedPipeServerStream CreateInstance(string name, bool first)
        {
            const uint PipeAccessDuplex = 3, Overlapped = 0x40000000, FirstInstance = 0x00080000;
            const uint RejectRemoteClients = 0x8;
            byte[] sd = CreateSecurity().GetSecurityDescriptorBinaryForm();
            GCHandle pin = GCHandle.Alloc(sd, GCHandleType.Pinned);
            try
            {
                var sa = new SecurityAttributes { Length = Marshal.SizeOf(typeof(SecurityAttributes)), Descriptor = pin.AddrOfPinnedObject(), Inherit = false };
                SafePipeHandle h = CreateNamedPipe("\\\\.\\pipe\\" + name, PipeAccessDuplex | Overlapped | (first ? FirstInstance : 0), RejectRemoteClients, MaxInstances, 4096, 4096, 0, ref sa);
                if (h.IsInvalid) throw new IOException("pipe name " + name + " is already in use or cannot be created", new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error()));
                return new NamedPipeServerStream(PipeDirection.InOut, true, false, h);
            }
            finally
            {
                pin.Free();
            }
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct SecurityAttributes
        {
            public int Length;
            public IntPtr Descriptor;
            [MarshalAs(UnmanagedType.Bool)] public bool Inherit;
        }

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern SafePipeHandle CreateNamedPipe(string name, uint openMode, uint pipeMode, uint maxInstances, uint outBuffer, uint inBuffer, uint defaultTimeout, ref SecurityAttributes security);

        private void AcceptLoop(NamedPipeServerStream next)
        {
            try
            {
                while (!_stop.WaitOne(0))
                {
                    NamedPipeServerStream pipe = null;
                    try
                    {
                        pipe = next ?? NewInstance(false);
                        next = null;
                        IAsyncResult ar = pipe.BeginWaitForConnection(null, null);
                        int w = WaitHandle.WaitAny(new[] { ar.AsyncWaitHandle, _stop });
                        if (w == 1) return; // finally disposes the pending instance
                        pipe.EndWaitForConnection(ar);
                        Accept(pipe);
                        pipe = null;
                    }
                    catch (Exception e)
                    {
                        if (_stop.WaitOne(0)) return;
                        if (++_errors <= 3) _log("SMBus bridge: accepting a connection failed: " + e.Message);
                        if (_stop.WaitOne(250)) return; // short back-off, then try again
                    }
                    finally
                    {
                        try { pipe?.Dispose(); } catch { }
                    }
                }
            }
            finally
            {
                _alive = false;
            }
        }

        // Runs on a timer, independent of the accept loop, so constant (re)connects
        // cannot keep idle connections alive.
        internal void Sweep()
        {
            try
            {
                int closed = _tracker.CloseIdle(_idleMs);
                if (closed > 0) _log("SMBus bridge: closed " + closed + " idle connection(s)");
            }
            catch (Exception) { }
        }

        private void Accept(NamedPipeServerStream pipe)
        {
            Stream stream = _tracker.TryAdd(pipe, ClientProcessId(pipe));
            if (stream == null)
            {
                pipe.Dispose(); // limit reached or shutting down
                return;
            }
            var t = new Thread(() => Handle(stream)) { IsBackground = true, Name = "smbus-bridge-client" };
            t.Start();
        }

        private void Handle(Stream stream)
        {
            try
            {
                _serve(stream);
            }
            catch (Exception)
            {
                // client went away or the pipe was closed by a disable or the idle timeout
            }
            finally
            {
                stream.Dispose(); // also unregisters it
            }
        }

        private static int ClientProcessId(NamedPipeServerStream pipe)
        {
            try
            {
                uint pid;
                return GetNamedPipeClientProcessId(pipe.SafePipeHandle.DangerousGetHandle(), out pid) ? (int)pid : 0;
            }
            catch (Exception)
            {
                return 0;
            }
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetNamedPipeClientProcessId(IntPtr pipe, out uint clientProcessId);

        public void Dispose()
        {
            _stop.Set();
            _sweepTimer?.Dispose();
            _tracker.CloseAll();
            _accept?.Join(2000);
        }
    }
}
