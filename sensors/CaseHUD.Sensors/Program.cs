// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

using System;
using System.Collections.Generic;
using System.Globalization;
using System.Net;
using System.ServiceProcess;
using System.Text;
using System.Threading;
using CaseHUD.Sensors.Smbus;
using LibreHardwareMonitor.Hardware;

namespace CaseHUD.Sensors
{
    /// <summary>
    /// Reads all hardware sensors via LibreHardwareMonitorLib and serves them as
    /// LibreHardwareMonitor-compatible JSON (data.json) on 127.0.0.1:8086.
    /// Run as the "CaseHUDSensors" Windows service, or with --console to debug.
    /// </summary>
    internal static class Program
    {
        public const string ServiceName = "CaseHUDSensors";
        public const string Prefix = "http://127.0.0.1:8086/";

        private static int Main(string[] args)
        {
            AppDomain.CurrentDomain.UnhandledException += (_, e) => Log.Write("crash: " + e.ExceptionObject);
            if (Array.IndexOf(args, "--console") >= 0)
            {
                using (var host = new SensorHost())
                {
                    host.Start();
                    Console.WriteLine("CaseHUD sensors running at " + Prefix + "data.json - press Enter to stop");
                    Console.ReadLine();
                }
                return 0;
            }
            ServiceBase.Run(new SensorService());
            return 0;
        }
    }

    internal sealed class SensorService : ServiceBase
    {
        private SensorHost _host;

        public SensorService()
        {
            ServiceName = Program.ServiceName;
            CanStop = true;
            CanShutdown = true;
        }

        protected override void OnStart(string[] args)
        {
            try
            {
                _host = new SensorHost();
                _host.Start();
            }
            catch (Exception e)
            {
                Log.Write("service start failed: " + e);
                throw;
            }
        }

        protected override void OnStop() => _host?.Dispose();

        protected override void OnShutdown() => _host?.Dispose();
    }

    internal sealed class SensorHost : IDisposable
    {
        private readonly Computer _computer = new Computer
        {
            IsCpuEnabled = true,
            IsGpuEnabled = true,
            IsMotherboardEnabled = true,
            IsControllerEnabled = true,
            IsMemoryEnabled = true,
            IsStorageEnabled = true,
            IsPsuEnabled = true,
        };

        private readonly HttpListener _listener = new HttpListener();
        private readonly object _lock = new object();
        private byte[] _json; // null until the first sensor pass is done
        private Thread _updater;
        private volatile bool _running;
        private SmbusBridgeHost _bridge;

        public void Start()
        {
            Log.Write("starting, " + Environment.OSVersion + ", 64-bit process: " + Environment.Is64BitProcess);
            _running = true;

            // Opening the hardware can take well over the 30 s Windows gives a
            // service to start (storage, RAM SPD, driver load), so it runs in
            // the background; until then requests get 503.
            _updater = new Thread(UpdateLoop) { IsBackground = true, Name = "sensor-update" };
            _updater.Start();

            try
            {
                _listener.Prefixes.Add(Program.Prefix);
                _listener.Start();
                _listener.BeginGetContext(OnRequest, null);
                Log.Write("listening on " + Program.Prefix);
            }
            catch (Exception e)
            {
                Log.Write("HTTP listener failed (port 8086 in use?): " + e);
                throw;
            }

            // Optional SMBus bridge for RAM RGB (off unless sensors.json enables it).
            try
            {
                _bridge = SmbusBridgeHost.CreateDefault(OpenSmbusModule, Log.Write);
                _bridge.Start();
            }
            catch (Exception e)
            {
                Log.Write("SMBus bridge setup failed: " + e.Message);
            }
        }

        // The SmbusPIIX4 PawnIO module is embedded in LibreHardwareMonitorLib.
        private static System.IO.Stream OpenSmbusModule() =>
            typeof(Computer).Assembly.GetManifestResourceStream("LibreHardwareMonitor.Resources.PawnIo.SmbusPIIX4.bin")
            ?? throw new InvalidOperationException("SmbusPIIX4 module not found in LibreHardwareMonitorLib");

        private void UpdateLoop()
        {
            var sw = System.Diagnostics.Stopwatch.StartNew();
            try
            {
                _computer.Open();
            }
            catch (Exception e)
            {
                Log.Write("opening hardware failed: " + e);
            }
            Log.Write("hardware opened in " + sw.ElapsedMilliseconds + " ms");

            var failed = new HashSet<string>();
            bool first = true;
            while (_running)
            {
                foreach (IHardware hw in _computer.Hardware)
                {
                    try
                    {
                        Update(hw);
                    }
                    catch (Exception e)
                    {
                        // A single failing device must not stop the others.
                        if (failed.Add(hw.Identifier.ToString())) Log.Write("update failed for " + hw.Name + ": " + e.Message);
                    }
                }
                try
                {
                    byte[] json = Encoding.UTF8.GetBytes(Json.Tree(_computer));
                    lock (_lock) _json = json;
                }
                catch (Exception e)
                {
                    Log.Write("building JSON failed: " + e);
                }
                if (first)
                {
                    first = false;
                    LogInventory();
                }
                Thread.Sleep(1000);
            }
        }

        private static void Update(IHardware hw)
        {
            hw.Update();
            foreach (IHardware sub in hw.SubHardware) Update(sub);
        }

        // Lists what was found once per start, so sensors.log answers "why is
        // sensor X missing" without a debugger.
        private void LogInventory()
        {
            foreach (IHardware hw in _computer.Hardware) LogHardware(hw, "  ");
        }

        private static void LogHardware(IHardware hw, string indent)
        {
            Log.Write(indent + hw.HardwareType + ": " + hw.Name + " (" + hw.Sensors.Length + " sensors)");
            foreach (ISensor s in hw.Sensors)
            {
                if (s.SensorType == SensorType.Temperature) Log.Write(indent + "  " + s.Name + " = " + (s.Value?.ToString("0.0", CultureInfo.InvariantCulture) ?? "-"));
            }
            foreach (IHardware sub in hw.SubHardware) LogHardware(sub, indent + "  ");
        }

        private void OnRequest(IAsyncResult ar)
        {
            HttpListenerContext ctx;
            try
            {
                ctx = _listener.EndGetContext(ar);
            }
            catch
            {
                return; // listener stopped
            }
            if (_running) _listener.BeginGetContext(OnRequest, null);

            try
            {
                byte[] body;
                lock (_lock) body = _json;
                ctx.Response.Headers["Cache-Control"] = "no-store";
                if (body == null)
                {
                    ctx.Response.StatusCode = 503; // still opening the hardware
                    return;
                }
                ctx.Response.ContentType = "application/json; charset=utf-8";
                ctx.Response.ContentLength64 = body.Length;
                ctx.Response.OutputStream.Write(body, 0, body.Length);
            }
            catch
            {
                // client went away
            }
            finally
            {
                try { ctx.Response.Close(); } catch { }
            }
        }

        public void Dispose()
        {
            _running = false;
            try { _bridge?.Dispose(); } catch { }
            try { _listener.Stop(); } catch { }
            try { _computer.Close(); } catch { }
            Log.Write("stopped");
        }
    }

    /// <summary>
    /// Plain-text log next to the service (Program Files\CaseHUD\sensors),
    /// restarted when it grows past 1 MB. The folder is writable only by
    /// administrators, so unprivileged users cannot redirect what the SYSTEM
    /// service writes or deletes (ProgramData would allow that).
    /// </summary>
    internal static class Log
    {
        private static readonly object Sync = new object();

        public static readonly string Path = System.IO.Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "sensors.log");

        public static void Write(string line)
        {
            lock (Sync)
            {
                try
                {
                    var fi = new System.IO.FileInfo(Path);
                    if (fi.Exists && fi.Length > 1 << 20) fi.Delete();
                    System.IO.File.AppendAllText(Path, DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss ", CultureInfo.InvariantCulture) + line + Environment.NewLine);
                }
                catch
                {
                    // logging must never take the service down
                }
                if (Environment.UserInteractive) Console.WriteLine(line);
            }
        }
    }

    /// <summary>Writes the sensor tree in LibreHardwareMonitor's data.json shape.</summary>
    internal static class Json
    {
        private static int _id;

        public static string Tree(IComputer computer)
        {
            _id = 0;
            var sb = new StringBuilder(64 * 1024);
            Open(sb, "Sensor", "");
            sb.Append(",\"Children\":[");
            Open(sb, Environment.MachineName, "");
            sb.Append(",\"Children\":[");
            bool first = true;
            foreach (IHardware hw in computer.Hardware)
            {
                if (!first) sb.Append(',');
                first = false;
                Hardware(sb, hw);
            }
            sb.Append("]}]}");
            return sb.ToString();
        }

        private static void Hardware(StringBuilder sb, IHardware hw)
        {
            Open(sb, hw.Name, "");
            sb.Append(",\"Children\":[");
            bool first = true;
            foreach (IHardware sub in hw.SubHardware)
            {
                if (!first) sb.Append(',');
                first = false;
                Hardware(sb, sub);
            }
            // Group sensors by type, like LibreHardwareMonitor's tree.
            var groups = new SortedDictionary<int, List<ISensor>>();
            foreach (ISensor s in hw.Sensors)
            {
                if (!groups.TryGetValue((int)s.SensorType, out var list)) groups[(int)s.SensorType] = list = new List<ISensor>();
                list.Add(s);
            }
            foreach (var g in groups)
            {
                if (!first) sb.Append(',');
                first = false;
                Open(sb, GroupName((SensorType)g.Key), "");
                sb.Append(",\"Children\":[");
                for (int i = 0; i < g.Value.Count; i++)
                {
                    if (i > 0) sb.Append(',');
                    ISensor s = g.Value[i];
                    Open(sb, s.Name, Format(s));
                    sb.Append(",\"SensorId\":").Append(Str(s.Identifier.ToString()));
                    sb.Append(",\"Type\":").Append(Str(s.SensorType.ToString()));
                    sb.Append(",\"Children\":[]}");
                }
                sb.Append("]}");
            }
            sb.Append("]}");
        }

        private static void Open(StringBuilder sb, string text, string value)
        {
            sb.Append("{\"id\":").Append(_id++).Append(",\"Text\":").Append(Str(text)).Append(",\"Value\":").Append(Str(value));
        }

        private static string GroupName(SensorType t)
        {
            switch (t)
            {
                case SensorType.Temperature: return "Temperatures";
                case SensorType.Load: return "Load";
                case SensorType.Fan: return "Fans";
                case SensorType.Power: return "Powers";
                case SensorType.Clock: return "Clocks";
                case SensorType.Voltage: return "Voltages";
                case SensorType.Control: return "Controls";
                case SensorType.Flow: return "Flows";
                case SensorType.Level: return "Levels";
                case SensorType.Data: return "Data";
                case SensorType.Frequency: return "Frequencies";
                case SensorType.Current: return "Currents";
                default: return t + "s";
            }
        }

        private static string Format(ISensor s)
        {
            if (s.Value == null) return "-";
            float v = s.Value.Value;
            // Newer LibreHardwareMonitorLib builds report memory sizes in
            // bytes instead of GB; show GB either way.
            if (s.SensorType == SensorType.Data && v > 1 << 20) v /= 1 << 30;
            string unit;
            switch (s.SensorType)
            {
                case SensorType.Temperature: unit = "°C"; break;
                case SensorType.Load:
                case SensorType.Control:
                case SensorType.Level:
                case SensorType.Humidity: unit = "%"; break;
                case SensorType.Fan: unit = "RPM"; break;
                case SensorType.Power: unit = "W"; break;
                case SensorType.Clock: unit = "MHz"; break;
                case SensorType.Voltage: unit = "V"; break;
                case SensorType.Flow: unit = "L/h"; break;
                case SensorType.Data: unit = "GB"; break;
                case SensorType.Frequency: unit = "Hz"; break;
                case SensorType.Current: unit = "A"; break;
                case SensorType.Throughput: unit = "B/s"; break;
                case SensorType.Timing: unit = "ns"; break;
                case SensorType.Energy: unit = "mWh"; break;
                case SensorType.Noise: unit = "dBA"; break;
                default: unit = ""; break;
            }
            string num = v.ToString("0.###", CultureInfo.InvariantCulture);
            return unit.Length > 0 ? num + " " + unit : num;
        }

        private static string Str(string s)
        {
            var sb = new StringBuilder(s.Length + 2).Append('"');
            foreach (char c in s)
            {
                switch (c)
                {
                    case '"': sb.Append("\\\""); break;
                    case '\\': sb.Append("\\\\"); break;
                    case '\n': sb.Append("\\n"); break;
                    case '\r': sb.Append("\\r"); break;
                    case '\t': sb.Append("\\t"); break;
                    default:
                        if (c < 0x20) sb.Append("\\u").Append(((int)c).ToString("x4"));
                        else sb.Append(c);
                        break;
                }
            }
            return sb.Append('"').ToString();
        }
    }
}
