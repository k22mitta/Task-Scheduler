import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Activity,
  CheckCircle2,
  Clock,
  Loader2,
  Plus,
  RefreshCw,
  Server,
  XCircle,
  Zap,
} from 'lucide-react';
import { AreaChart, Area, ResponsiveContainer, Tooltip } from 'recharts';
import type { Job, JobStatus, NewJobForm, WSMessage } from './types';

const API = 'http://localhost:8080';
const WS_URL = 'ws://localhost:8080/ws';

const STATUS_STYLES: Record<JobStatus, string> = {
  pending:  'bg-yellow-500/15 text-yellow-400 ring-1 ring-yellow-500/30',
  running:  'bg-blue-500/15  text-blue-400  ring-1 ring-blue-500/30',
  done:     'bg-emerald-500/15 text-emerald-400 ring-1 ring-emerald-500/30',
  failed:   'bg-red-500/15   text-red-400   ring-1 ring-red-500/30',
  dead:     'bg-zinc-500/15  text-zinc-400  ring-1 ring-zinc-500/30',
};

interface ActivityEvent {
  id: number;
  icon: string;
  text: string;
  sub: string;
  time: Date;
  color: string;
}

interface MinuteBucket {
  label: string;
  done: number;
}

function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS_STYLES[status]}`}>
      {status === 'running' && (
        <span className="relative flex h-1.5 w-1.5">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-75" />
          <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-blue-400" />
        </span>
      )}
      {status}
    </span>
  );
}

function StatCard({ label, value, sub, icon: Icon, accent }: {
  label: string; value: string | number; sub?: string; icon: React.ElementType; accent: string;
}) {
  return (
    <div className="rounded-xl border border-white/8 bg-[#13151c] p-4">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs text-zinc-500 uppercase tracking-wider">{label}</p>
          <p className="mt-1.5 text-2xl font-bold text-white">{value}</p>
          {sub && <p className="mt-0.5 text-xs text-zinc-500">{sub}</p>}
        </div>
        <div className={`rounded-xl p-2.5 ${accent}`}><Icon size={15} /></div>
      </div>
    </div>
  );
}

function formatTime(d: Date) {
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function formatDate(iso: string | null) {
  if (!iso) return '—';
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' });
}

function jobDuration(job: Job) {
  if (!job.started_at || !job.finished_at) return null;
  const ms = new Date(job.finished_at).getTime() - new Date(job.started_at).getTime();
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
}

function buildBuckets(completedAt: number[]): MinuteBucket[] {
  const now = Date.now();
  const buckets: MinuteBucket[] = Array.from({ length: 10 }, (_, i) => {
    const t = new Date(now - (9 - i) * 60_000);
    return { label: `${t.getHours()}:${String(t.getMinutes()).padStart(2, '0')}`, done: 0 };
  });
  completedAt.forEach((ts) => {
    const age = Math.floor((now - ts) / 60_000);
    if (age >= 0 && age < 10) buckets[9 - age].done++;
  });
  return buckets;
}

const EMPTY_FORM: NewJobForm = {
  name: '',
  payload: '{}',
  scheduled_at: new Date(Date.now() + 60_000).toISOString().slice(0, 16),
  max_attempts: 3,
  cron_expression: '',
};

let eventSeq = 0;

export default function App() {
  const [jobs, setJobs]               = useState<Job[]>([]);
  const [loading, setLoading]         = useState(true);
  const [wsStatus, setWsStatus]       = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [showModal, setShowModal]     = useState(false);
  const [form, setForm]               = useState<NewJobForm>(EMPTY_FORM);
  const [submitting, setSubmitting]   = useState(false);
  const [formError, setFormError]     = useState('');
  const [feed, setFeed]               = useState<ActivityEvent[]>([]);
  const [totalEvents, setTotalEvents] = useState(0);
  const [processed, setProcessed]    = useState(0);
  const [flashMap, setFlashMap]       = useState<Record<string, string>>({});
  const [completedAt, setCompletedAt] = useState<number[]>([]);
  const wsRef     = useRef<WebSocket | null>(null);
  const feedRef   = useRef<HTMLDivElement>(null);

  const fetchJobs = useCallback(async () => {
    try {
      const res  = await fetch(`${API}/jobs?limit=100`);
      const data: Job[] = await res.json();
      setJobs(data);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchJobs(); }, [fetchJobs]);

  useEffect(() => {
    function connect() {
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;
      ws.onopen  = () => setWsStatus('connected');
      ws.onclose = () => { setWsStatus('disconnected'); setTimeout(connect, 3000); };
      ws.onerror = () => ws.close();
      ws.onmessage = (e) => {
        const msg: WSMessage = JSON.parse(e.data);
        setTotalEvents((n) => n + 1);

        const statusMap: Record<WSMessage['type'], JobStatus> = {
          'job.running': 'running',
          'job.done':    'done',
          'job.failed':  'failed',
        };
        const newStatus = statusMap[msg.type];

        setJobs((prev) => prev.map((j) => j.id === msg.job_id ? { ...j, status: newStatus } : j));

        const flashColor = newStatus === 'running' ? 'flash-yellow' : newStatus === 'done' ? 'flash-green' : 'flash-red';
        setFlashMap((prev) => ({ ...prev, [msg.job_id]: flashColor }));
        setTimeout(() => setFlashMap((prev) => { const n = { ...prev }; delete n[msg.job_id]; return n; }), 800);

        let icon = '⚡', text = '', color = 'text-blue-400';
        if (msg.type === 'job.done') {
          icon  = '✓';
          text  = msg.duration_ms != null ? `${msg.name} completed in ${msg.duration_ms < 1000 ? `${msg.duration_ms}ms` : `${(msg.duration_ms / 1000).toFixed(1)}s`}` : `${msg.name} completed`;
          color = 'text-emerald-400';
          setProcessed((n) => n + 1);
          setCompletedAt((prev) => [...prev.slice(-599), Date.now()]);
        } else if (msg.type === 'job.failed') {
          icon  = '✗';
          text  = `${msg.name} failed`;
          color = 'text-red-400';
        } else {
          text  = `${msg.name} started`;
        }

        const ev: ActivityEvent = { id: ++eventSeq, icon, text, sub: msg.job_id.slice(0, 8), time: new Date(), color };
        setFeed((prev) => [ev, ...prev].slice(0, 50));
      };
    }
    connect();
    return () => wsRef.current?.close();
  }, []);

  const counts = jobs.reduce(
    (acc, j) => ({ ...acc, [j.status]: (acc[j.status] ?? 0) + 1 }),
    {} as Record<JobStatus, number>,
  );
  const terminal   = (counts.done ?? 0) + (counts.failed ?? 0) + (counts.dead ?? 0);
  const successRate = terminal === 0 ? '—' : `${Math.round(((counts.done ?? 0) / terminal) * 100)}%`;

  const buckets = buildBuckets(completedAt);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError('');
    setSubmitting(true);
    try {
      let payload: unknown;
      try { payload = JSON.parse(form.payload); }
      catch { setFormError('Payload must be valid JSON'); return; }

      const body: Record<string, unknown> = {
        name: form.name, payload,
        scheduled_at: new Date(form.scheduled_at).toISOString(),
        max_attempts: form.max_attempts,
      };
      if (form.cron_expression) body.cron_expression = form.cron_expression;

      const res = await fetch(`${API}/jobs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const err = await res.json();
        setFormError(err.error ?? 'Failed to create job');
        return;
      }
      const created: Job = await res.json();
      setJobs((prev) => [created, ...prev]);
      setShowModal(false);
      setForm(EMPTY_FORM);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="h-screen overflow-hidden bg-[#0b0d12] text-white" style={{ fontFamily: "'Inter', system-ui, sans-serif" }}>
      <div className="flex h-full">

        <aside className="flex w-52 shrink-0 flex-col border-r border-white/6 bg-[#080a0e] px-3 py-5">
          <div className="mb-6 flex items-center gap-2.5 px-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-500 shadow-lg shadow-indigo-500/30">
              <Zap size={13} className="text-white" />
            </div>
            <span className="text-sm font-semibold tracking-tight">TaskScheduler</span>
          </div>

          <nav className="space-y-0.5 text-sm">
            <a href="#" className="flex items-center gap-2.5 rounded-lg bg-indigo-500/12 px-3 py-2 font-medium text-indigo-400">
              <Activity size={14} />Dashboard
            </a>
          </nav>

          <div className="mt-6 rounded-xl border border-white/6 bg-white/3 p-3 space-y-3">
            <p className="text-[10px] uppercase tracking-widest text-zinc-600 font-medium">Node Status</p>
            <div className="flex items-center gap-2">
              <div className={`h-2 w-2 rounded-full shadow-sm ${
                wsStatus === 'connected'    ? 'bg-emerald-400 shadow-emerald-400/50' :
                wsStatus === 'connecting'   ? 'bg-yellow-400 animate-pulse shadow-yellow-400/50' :
                                              'bg-red-400 shadow-red-400/50'
              }`} />
              <span className="text-xs capitalize text-zinc-300">{wsStatus}</span>
            </div>
            <div className="space-y-1.5">
              {[
                ['Events received', totalEvents.toLocaleString()],
                ['Jobs processed', processed.toLocaleString()],
              ].map(([k, v]) => (
                <div key={k} className="flex items-center justify-between">
                  <span className="text-[11px] text-zinc-500">{k}</span>
                  <span className="text-[11px] font-medium text-zinc-300">{v}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="mt-3 rounded-xl border border-white/6 bg-white/3 p-3">
            <p className="mb-2 text-[10px] uppercase tracking-widest text-zinc-600 font-medium">Completions / min</p>
            <ResponsiveContainer width="100%" height={48}>
              <AreaChart data={buckets} margin={{ top: 2, right: 0, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="sparkGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%"   stopColor="#6366f1" stopOpacity={0.4} />
                    <stop offset="100%" stopColor="#6366f1" stopOpacity={0}   />
                  </linearGradient>
                </defs>
                <Area type="monotone" dataKey="done" stroke="#6366f1" strokeWidth={1.5} fill="url(#sparkGrad)" dot={false} />
                <Tooltip
                  contentStyle={{ background: '#13151c', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8, fontSize: 11 }}
                  labelStyle={{ color: '#71717a' }}
                  itemStyle={{ color: '#a5b4fc' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          <div className="mt-auto pt-4">
            <div className="flex items-center gap-2 px-2">
              <Server size={12} className="text-zinc-600" />
              <span className="text-[11px] text-zinc-600">localhost:8080</span>
            </div>
          </div>
        </aside>

        <div className="flex flex-1 flex-col overflow-hidden">
          <header className="flex shrink-0 items-center justify-between border-b border-white/6 bg-[#0b0d12]/80 px-5 py-3.5 backdrop-blur">
            <div>
              <h1 className="text-base font-semibold">Jobs Dashboard</h1>
              <p className="text-[11px] text-zinc-500">Real-time distributed job monitoring</p>
            </div>
            <div className="flex items-center gap-2">
              <button onClick={fetchJobs} className="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/4 px-3 py-1.5 text-xs text-zinc-300 transition hover:bg-white/8">
                <RefreshCw size={11} />Refresh
              </button>
              <button onClick={() => setShowModal(true)} className="flex items-center gap-1.5 rounded-lg bg-indigo-500 px-3 py-1.5 text-xs font-medium text-white shadow-lg shadow-indigo-500/20 transition hover:bg-indigo-400">
                <Plus size={11} />New Job
              </button>
            </div>
          </header>

          <div className="grid grid-cols-6 gap-2.5 shrink-0 px-5 py-3">
            <StatCard label="Total"        value={jobs.length}           icon={Activity}    accent="bg-indigo-500/15 text-indigo-400" />
            <StatCard label="Pending"      value={counts.pending  ?? 0}  icon={Clock}       accent="bg-yellow-500/15 text-yellow-400" />
            <StatCard label="Running"      value={counts.running  ?? 0}  icon={Loader2}     accent="bg-blue-500/15 text-blue-400" />
            <StatCard label="Done"         value={counts.done     ?? 0}  icon={CheckCircle2} accent="bg-emerald-500/15 text-emerald-400" />
            <StatCard label="Failed/Dead"  value={(counts.failed ?? 0) + (counts.dead ?? 0)} icon={XCircle} accent="bg-red-500/15 text-red-400" />
            <StatCard label="Success Rate" value={successRate}            icon={Activity}    accent="bg-violet-500/15 text-violet-400" sub={terminal > 0 ? `${terminal} resolved` : undefined} />
          </div>

          <div className="flex flex-1 gap-3 overflow-hidden px-5 pb-4">
            <div className="flex flex-1 flex-col overflow-hidden rounded-xl border border-white/6 bg-[#0e1016]">
              <div className="flex items-center justify-between border-b border-white/6 px-4 py-2.5">
                <span className="text-xs font-medium text-zinc-400">All Jobs</span>
                <span className="rounded-full bg-white/5 px-2 py-0.5 text-[10px] text-zinc-500">{jobs.length}</span>
              </div>
              <div className="flex-1 overflow-auto">
                <table className="w-full text-xs">
                  <thead className="sticky top-0 bg-[#0e1016]">
                    <tr className="border-b border-white/5">
                      {['Job', 'Status', 'Scheduled', 'Attempts', 'Duration'].map((h) => (
                        <th key={h} className="px-4 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-zinc-600">{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {loading ? (
                      <tr><td colSpan={5} className="px-4 py-10 text-center text-zinc-600"><Loader2 size={18} className="mx-auto animate-spin" /></td></tr>
                    ) : jobs.length === 0 ? (
                      <tr><td colSpan={5} className="px-4 py-10 text-center text-zinc-600">No jobs yet — create one to get started</td></tr>
                    ) : jobs.map((job) => {
                      const flash = flashMap[job.id];
                      const rowBg = flash === 'flash-yellow' ? 'bg-yellow-500/8' : flash === 'flash-green' ? 'bg-emerald-500/8' : flash === 'flash-red' ? 'bg-red-500/8' : '';
                      const dur = jobDuration(job);
                      return (
                        <tr key={job.id} className={`border-b border-white/4 transition-colors duration-700 hover:bg-white/3 ${rowBg}`}>
                          <td className="px-4 py-2.5">
                            <div className="font-medium text-white">{job.name}</div>
                            <div className="font-mono text-[10px] text-zinc-600">{job.id.slice(0, 8)}…</div>
                          </td>
                          <td className="px-4 py-2.5"><StatusBadge status={job.status} /></td>
                          <td className="px-4 py-2.5 text-zinc-500">{formatDate(job.scheduled_at)}</td>
                          <td className="px-4 py-2.5">
                            <div className="flex items-center gap-1.5">
                              <div className="h-1 flex-1 max-w-[40px] rounded-full bg-white/8">
                                <div
                                  className="h-full rounded-full bg-indigo-500"
                                  style={{ width: `${Math.min(100, (job.attempts / job.max_attempts) * 100)}%` }}
                                />
                              </div>
                              <span className="text-zinc-500">{job.attempts}/{job.max_attempts}</span>
                            </div>
                          </td>
                          <td className="px-4 py-2.5 text-zinc-500">{dur ?? '—'}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="flex w-64 shrink-0 flex-col overflow-hidden rounded-xl border border-white/6 bg-[#0e1016]">
              <div className="flex items-center justify-between border-b border-white/6 px-4 py-2.5">
                <span className="text-xs font-medium text-zinc-400">Live Activity</span>
                <span className="flex items-center gap-1 text-[10px] text-zinc-600">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                  live
                </span>
              </div>
              <div ref={feedRef} className="flex-1 overflow-y-auto p-2 space-y-1">
                {feed.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-[11px] text-zinc-600">Waiting for events…</div>
                ) : feed.map((ev) => (
                  <div key={ev.id} className="flex items-start gap-2 rounded-lg px-2.5 py-2 transition hover:bg-white/3">
                    <span className={`mt-0.5 shrink-0 text-sm leading-none ${ev.color}`}>{ev.icon}</span>
                    <div className="min-w-0">
                      <p className={`text-[11px] font-medium leading-tight ${ev.color}`}>{ev.text}</p>
                      <div className="mt-0.5 flex items-center gap-1.5">
                        <span className="font-mono text-[10px] text-zinc-600">{ev.sub}</span>
                        <span className="text-[10px] text-zinc-600">{formatTime(ev.time)}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>

      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-white/10 bg-[#0e1016] p-6 shadow-2xl">
            <div className="mb-5 flex items-center justify-between">
              <h2 className="text-sm font-semibold">New Job</h2>
              <button onClick={() => { setShowModal(false); setFormError(''); }} className="text-zinc-600 hover:text-white transition">
                <XCircle size={17} />
              </button>
            </div>
            <form onSubmit={handleSubmit} className="space-y-3.5">
              {[
                { label: 'Name', key: 'name', type: 'text', placeholder: 'send_email' },
              ].map(({ label, key, type, placeholder }) => (
                <div key={key}>
                  <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-zinc-500">{label}</label>
                  <input
                    required type={type}
                    value={(form as unknown as Record<string, string | number>)[key] as string}
                    onChange={(e) => setForm({ ...form, [key]: e.target.value })}
                    placeholder={placeholder}
                    className="w-full rounded-lg border border-white/8 bg-white/4 px-3 py-2 text-sm text-white placeholder-zinc-700 outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40"
                  />
                </div>
              ))}
              <div>
                <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-zinc-500">Payload (JSON)</label>
                <textarea rows={3} value={form.payload}
                  onChange={(e) => setForm({ ...form, payload: e.target.value })}
                  className="w-full rounded-lg border border-white/8 bg-white/4 px-3 py-2 font-mono text-sm text-white placeholder-zinc-700 outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40"
                />
              </div>
              <div>
                <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-zinc-500">Scheduled At</label>
                <input required type="datetime-local" value={form.scheduled_at}
                  onChange={(e) => setForm({ ...form, scheduled_at: e.target.value })}
                  className="w-full rounded-lg border border-white/8 bg-white/4 px-3 py-2 text-sm text-white outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-zinc-500">Max Attempts</label>
                  <input type="number" min={1} max={10} value={form.max_attempts}
                    onChange={(e) => setForm({ ...form, max_attempts: Number(e.target.value) })}
                    className="w-full rounded-lg border border-white/8 bg-white/4 px-3 py-2 text-sm text-white outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40"
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-zinc-500">Cron (optional)</label>
                  <input value={form.cron_expression}
                    onChange={(e) => setForm({ ...form, cron_expression: e.target.value })}
                    placeholder="* * * * *"
                    className="w-full rounded-lg border border-white/8 bg-white/4 px-3 py-2 font-mono text-sm text-white placeholder-zinc-700 outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40"
                  />
                </div>
              </div>
              {formError && <p className="text-xs text-red-400">{formError}</p>}
              <div className="flex gap-2 pt-1">
                <button type="button" onClick={() => { setShowModal(false); setFormError(''); }}
                  className="flex-1 rounded-lg border border-white/8 bg-white/4 py-2 text-xs text-zinc-400 transition hover:bg-white/8">
                  Cancel
                </button>
                <button type="submit" disabled={submitting}
                  className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-indigo-500 py-2 text-xs font-medium text-white shadow-lg shadow-indigo-500/20 transition hover:bg-indigo-400 disabled:opacity-50">
                  {submitting ? <Loader2 size={12} className="animate-spin" /> : <Plus size={12} />}Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
