import { useEffect, useRef, useState } from 'react';
import {
  Activity,
  CheckCircle2,
  Clock,
  Loader2,
  Plus,
  RefreshCw,
  XCircle,
  Zap,
} from 'lucide-react';
import type { Job, JobStatus, NewJobForm, WSMessage } from './types';

const API = 'http://localhost:8080';
const WS_URL = 'ws://localhost:8080/ws';

const STATUS_STYLES: Record<JobStatus, string> = {
  pending: 'bg-yellow-500/15 text-yellow-400 ring-1 ring-yellow-500/30',
  running: 'bg-blue-500/15 text-blue-400 ring-1 ring-blue-500/30',
  done: 'bg-emerald-500/15 text-emerald-400 ring-1 ring-emerald-500/30',
  failed: 'bg-red-500/15 text-red-400 ring-1 ring-red-500/30',
  dead: 'bg-zinc-500/15 text-zinc-400 ring-1 ring-zinc-500/30',
};

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

function StatCard({ label, value, icon: Icon, color }: { label: string; value: number; icon: React.ElementType; color: string }) {
  return (
    <div className="rounded-xl border border-white/8 bg-white/4 p-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-zinc-400">{label}</p>
        <div className={`rounded-lg p-2 ${color}`}>
          <Icon size={14} />
        </div>
      </div>
      <p className="mt-2 text-2xl font-semibold text-white">{value}</p>
    </div>
  );
}

function formatDate(iso: string | null) {
  if (!iso) return '—';
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
}

function duration(job: Job) {
  if (!job.started_at || !job.finished_at) return '—';
  const ms = new Date(job.finished_at).getTime() - new Date(job.started_at).getTime();
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
}

const EMPTY_FORM: NewJobForm = {
  name: '',
  payload: '{}',
  scheduled_at: new Date(Date.now() + 60_000).toISOString().slice(0, 16),
  max_attempts: 3,
  cron_expression: '',
};

export default function App() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState<NewJobForm>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');
  const wsRef = useRef<WebSocket | null>(null);

  const fetchJobs = async () => {
    try {
      const res = await fetch(`${API}/jobs?limit=100`);
      const data: Job[] = await res.json();
      setJobs(data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchJobs();
  }, []);

  useEffect(() => {
    function connect() {
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onopen = () => setWsStatus('connected');
      ws.onclose = () => {
        setWsStatus('disconnected');
        setTimeout(connect, 3000);
      };
      ws.onerror = () => ws.close();

      ws.onmessage = (e) => {
        const msg: WSMessage = JSON.parse(e.data);
        const statusMap: Record<WSMessage['type'], JobStatus> = {
          'job.running': 'running',
          'job.done': 'done',
          'job.failed': 'failed',
        };
        setJobs((prev) =>
          prev.map((j) =>
            j.id === msg.job_id ? { ...j, status: statusMap[msg.type] } : j
          )
        );
      };
    }

    connect();
    return () => wsRef.current?.close();
  }, []);

  const counts = jobs.reduce(
    (acc, j) => ({ ...acc, [j.status]: (acc[j.status] ?? 0) + 1 }),
    {} as Record<JobStatus, number>
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError('');
    setSubmitting(true);
    try {
      let payload: unknown;
      try {
        payload = JSON.parse(form.payload);
      } catch {
        setFormError('Payload must be valid JSON');
        return;
      }
      const body: Record<string, unknown> = {
        name: form.name,
        payload,
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
    <div className="min-h-screen bg-[#0f1117] text-white" style={{ fontFamily: "'Inter', system-ui, sans-serif" }}>
      <div className="flex h-screen overflow-hidden">
        <aside className="flex w-56 flex-col border-r border-white/8 bg-[#0a0c10] px-4 py-6">
          <div className="mb-8 flex items-center gap-2.5 px-1">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-500">
              <Zap size={14} className="text-white" />
            </div>
            <span className="text-sm font-semibold tracking-tight">TaskScheduler</span>
          </div>
          <nav className="space-y-0.5">
            <a href="#" className="flex items-center gap-2.5 rounded-lg bg-indigo-500/15 px-3 py-2 text-sm font-medium text-indigo-400">
              <Activity size={15} />
              Dashboard
            </a>
          </nav>
          <div className="mt-auto flex items-center gap-2 rounded-lg px-3 py-2">
            <div className={`h-1.5 w-1.5 rounded-full ${wsStatus === 'connected' ? 'bg-emerald-400' : wsStatus === 'connecting' ? 'bg-yellow-400 animate-pulse' : 'bg-red-400'}`} />
            <span className="text-xs text-zinc-500 capitalize">{wsStatus}</span>
          </div>
        </aside>

        <main className="flex flex-1 flex-col overflow-hidden">
          <header className="flex items-center justify-between border-b border-white/8 px-6 py-4">
            <div>
              <h1 className="text-lg font-semibold">Jobs Dashboard</h1>
              <p className="text-xs text-zinc-500">{jobs.length} total jobs</p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={fetchJobs}
                className="flex items-center gap-1.5 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-zinc-300 transition hover:bg-white/10"
              >
                <RefreshCw size={13} />
                Refresh
              </button>
              <button
                onClick={() => setShowModal(true)}
                className="flex items-center gap-1.5 rounded-lg bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-indigo-400"
              >
                <Plus size={13} />
                New Job
              </button>
            </div>
          </header>

          <div className="grid grid-cols-5 gap-3 px-6 py-4">
            <StatCard label="Total" value={jobs.length} icon={Activity} color="bg-indigo-500/15 text-indigo-400" />
            <StatCard label="Pending" value={counts.pending ?? 0} icon={Clock} color="bg-yellow-500/15 text-yellow-400" />
            <StatCard label="Running" value={counts.running ?? 0} icon={Loader2} color="bg-blue-500/15 text-blue-400" />
            <StatCard label="Done" value={counts.done ?? 0} icon={CheckCircle2} color="bg-emerald-500/15 text-emerald-400" />
            <StatCard label="Failed / Dead" value={(counts.failed ?? 0) + (counts.dead ?? 0)} icon={XCircle} color="bg-red-500/15 text-red-400" />
          </div>

          <div className="flex-1 overflow-auto px-6 pb-6">
            <div className="rounded-xl border border-white/8 bg-white/3">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-white/8">
                    {['Name', 'Status', 'Scheduled At', 'Attempts', 'Duration'].map((h) => (
                      <th key={h} className="px-4 py-3 text-left text-xs font-medium text-zinc-500">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-12 text-center text-zinc-500">
                        <Loader2 size={20} className="mx-auto animate-spin" />
                      </td>
                    </tr>
                  ) : jobs.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-12 text-center text-zinc-500">No jobs yet</td>
                    </tr>
                  ) : (
                    jobs.map((job) => (
                      <tr key={job.id} className="border-b border-white/5 transition hover:bg-white/3">
                        <td className="px-4 py-3">
                          <div className="font-medium text-white">{job.name}</div>
                          <div className="text-xs text-zinc-500 font-mono">{job.id.slice(0, 8)}…</div>
                        </td>
                        <td className="px-4 py-3"><StatusBadge status={job.status} /></td>
                        <td className="px-4 py-3 text-zinc-400">{formatDate(job.scheduled_at)}</td>
                        <td className="px-4 py-3 text-zinc-400">
                          {job.attempts} / {job.max_attempts}
                        </td>
                        <td className="px-4 py-3 text-zinc-400">{duration(job)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </main>
      </div>

      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-white/10 bg-[#13151c] p-6 shadow-2xl">
            <div className="mb-5 flex items-center justify-between">
              <h2 className="text-base font-semibold">New Job</h2>
              <button onClick={() => { setShowModal(false); setFormError(''); }} className="text-zinc-500 hover:text-white">
                <XCircle size={18} />
              </button>
            </div>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="mb-1.5 block text-xs font-medium text-zinc-400">Name</label>
                <input
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white placeholder-zinc-600 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="send_email"
                />
              </div>
              <div>
                <label className="mb-1.5 block text-xs font-medium text-zinc-400">Payload (JSON)</label>
                <textarea
                  rows={3}
                  value={form.payload}
                  onChange={(e) => setForm({ ...form, payload: e.target.value })}
                  className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 font-mono text-sm text-white placeholder-zinc-600 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label className="mb-1.5 block text-xs font-medium text-zinc-400">Scheduled At</label>
                <input
                  required
                  type="datetime-local"
                  value={form.scheduled_at}
                  onChange={(e) => setForm({ ...form, scheduled_at: e.target.value })}
                  className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-zinc-400">Max Attempts</label>
                  <input
                    type="number"
                    min={1}
                    max={10}
                    value={form.max_attempts}
                    onChange={(e) => setForm({ ...form, max_attempts: Number(e.target.value) })}
                    className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-white outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-zinc-400">Cron (optional)</label>
                  <input
                    value={form.cron_expression}
                    onChange={(e) => setForm({ ...form, cron_expression: e.target.value })}
                    className="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2 font-mono text-sm text-white placeholder-zinc-600 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                    placeholder="* * * * *"
                  />
                </div>
              </div>
              {formError && <p className="text-xs text-red-400">{formError}</p>}
              <div className="flex gap-2 pt-1">
                <button
                  type="button"
                  onClick={() => { setShowModal(false); setFormError(''); }}
                  className="flex-1 rounded-lg border border-white/10 bg-white/5 py-2 text-sm text-zinc-300 hover:bg-white/10"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-indigo-500 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:opacity-50"
                >
                  {submitting ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
