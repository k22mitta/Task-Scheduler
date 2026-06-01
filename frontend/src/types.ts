export type JobStatus = 'pending' | 'running' | 'done' | 'failed' | 'dead';

export interface Job {
  id: string;
  name: string;
  payload: Record<string, unknown>;
  status: JobStatus;
  scheduled_at: string;
  started_at: string | null;
  finished_at: string | null;
  attempts: number;
  max_attempts: number;
  cron_expression: string | null;
  created_at: string;
  updated_at: string;
}

export interface JobRun {
  id: string;
  job_id: string;
  attempt: number;
  started_at: string;
  finished_at: string | null;
  status: JobStatus;
  error_message: string | null;
}

export interface WSMessage {
  type: 'job.running' | 'job.done' | 'job.failed';
  job_id: string;
  name: string;
  duration_ms?: number;
}

export interface NewJobForm {
  name: string;
  payload: string;
  scheduled_at: string;
  max_attempts: number;
  cron_expression: string;
}
