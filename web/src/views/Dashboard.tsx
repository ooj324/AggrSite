import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { format } from 'date-fns';

export default function Dashboard() {
  const [status, setStatus] = useState<SchedulerStatus | null>(null);
  const [stats, setStats] = useState({ sites: 0, accounts: 0 });
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);

  const loadData = async () => {
    try {
      const [schedRes, sitesRes, accountsRes] = await Promise.all([
        api.get('/api/scheduler/status'),
        api.get('/api/sites'),
        api.get('/api/accounts'),
      ]);
      setStatus(schedRes.data);
      setStats({
        sites: sitesRes.data?.length || 0,
        accounts: accountsRes.data?.length || 0,
      });
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleRunAllCheckins = async () => {
    if (!confirm('Run all checkins now?')) return;
    setActionLoading(true);
    try {
      await api.post('/api/checkin/all');
      alert('All checkins triggered successfully.');
    } catch (err: any) {
      alert(`Error: ${err}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleRefreshAllBalances = async () => {
    if (!confirm('Refresh all balances now?')) return;
    setActionLoading(true);
    try {
      await api.post('/api/balance/refresh/all');
      alert('All balances refreshed successfully.');
    } catch (err: any) {
      alert(`Error: ${err}`);
    } finally {
      setActionLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="animate-fade-in" style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
        <span className="spinner spinner-lg text-primary" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h2 className="greeting">
          Dashboard
        </h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={handleRunAllCheckins} disabled={actionLoading} className="btn btn-primary">
            Run Checkins
          </button>
          <button onClick={handleRefreshAllBalances} disabled={actionLoading} className="btn btn-soft-primary">
            {actionLoading ? 'Refreshing...' : 'Refresh Balances'}
          </button>
        </div>
      </div>

      <div className="dashboard-stat-grid">
        <div className="stat-card animate-slide-up stagger-1">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
            </svg>
            System Data
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-blue">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">Total Sites</div>
              <div className="stat-value animate-count-up">{stats.sites}</div>
            </div>
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-green">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">Total Accounts</div>
              <div className="stat-value animate-count-up">{stats.accounts}</div>
            </div>
          </div>
        </div>

        <div className="stat-card animate-slide-up stagger-2">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Scheduler
          </div>
          <div className="stat-card-row">
            <div className={`stat-icon ${status?.running ? 'stat-icon-green' : 'stat-icon-red'}`}>
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">Status</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 16 }}>
                {status?.running ? 'Running' : 'Stopped'}
              </div>
            </div>
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-yellow">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">Next Checkin</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 16 }}>
                {status?.next_checkin ? format(new Date(status.next_checkin), 'HH:mm') : 'None'}
              </div>
              <div style={{ fontSize: 11, color: "var(--color-text-muted)", marginTop: 2 }}>
                Cron: {status?.checkin_cron}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
