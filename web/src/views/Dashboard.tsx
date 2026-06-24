import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { format } from 'date-fns';

export default function Dashboard() {
  const [status, setStatus] = useState<SchedulerStatus | null>(null);
  const [stats, setStats] = useState({ 
    sites: 0, 
    accounts: 0, 
    total_balance: 0, 
    total_used: 0, 
    checkins_today: 0, 
    checkins_success: 0, 
    checkin_rate: 0 
  });
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);

  const loadData = async () => {
    try {
      const [schedRes, statsRes] = await Promise.all([
        api.get('/api/scheduler/status'),
        api.get('/api/stats/dashboard')
      ]);
      setStatus(schedRes.data);
      setStats(statsRes.data || {
        sites: 0, accounts: 0, total_balance: 0, total_used: 0, checkins_today: 0, checkins_success: 0, checkin_rate: 0
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
    if (!confirm('确定要现在执行所有签到吗？')) return;
    setActionLoading(true);
    try {
      await api.post('/api/checkin/all');
      alert('所有签到已成功触发。');
    } catch (err: any) {
      alert(`错误: ${err}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleRefreshAllBalances = async () => {
    if (!confirm('确定要现在刷新所有余额吗？')) return;
    setActionLoading(true);
    try {
      await api.post('/api/balance/refresh/all');
      alert('所有余额已成功刷新。');
    } catch (err: any) {
      alert(`错误: ${err}`);
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
          仪表盘
        </h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={handleRunAllCheckins} disabled={actionLoading} className="btn btn-primary">
            执行签到
          </button>
          <button onClick={handleRefreshAllBalances} disabled={actionLoading} className="btn btn-soft-primary">
            {actionLoading ? '刷新中...' : '刷新余额'}
          </button>
        </div>
      </div>

      <div className="dashboard-stat-grid">
        <div className="stat-card animate-slide-up stagger-1">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
            </svg>
            系统数据
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-blue">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">站点总数</div>
              <div className="stat-value animate-count-up">{stats.sites}</div>
            </div>
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-green">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 0-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">账户总数</div>
              <div className="stat-value animate-count-up">{stats.accounts}</div>
            </div>
          </div>
        </div>

        <div className="stat-card animate-slide-up stagger-2">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            账户数据
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-blue">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">总余额</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 18 }}>${stats.total_balance?.toFixed(2) || '0.00'}</div>
            </div>
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-green">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">总消耗</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 18 }}>${stats.total_used?.toFixed(2) || '0.00'}</div>
            </div>
          </div>
        </div>

        <div className="stat-card animate-slide-up stagger-3">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            签到状态
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-purple">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">今日签到</div>
              <div className="stat-value animate-count-up">{stats.checkins_success} / {stats.checkins_today}</div>
            </div>
          </div>
          <div className="stat-card-row">
            <div className="stat-icon stat-icon-orange">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">成功率</div>
              <div className="stat-value animate-count-up">{Math.round(stats.checkin_rate)}%</div>
            </div>
          </div>
        </div>

        <div className="stat-card animate-slide-up stagger-4">
          <div className="stat-card-header">
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            调度器
          </div>
          <div className="stat-card-row">
            <div className={`stat-icon ${status?.running ? 'stat-icon-green' : 'stat-icon-red'}`}>
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <div className="dashboard-stat-content">
              <div className="stat-label">状态</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 16 }}>
                {status?.running ? '运行中' : '已停止'}
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
              <div className="stat-label">下次签到</div>
              <div className="stat-value animate-count-up" style={{ fontSize: 16 }}>
                {status?.next_checkin ? format(new Date(status.next_checkin), 'HH:mm') : '无'}
              </div>
              <div style={{ fontSize: 11, color: "var(--color-text-muted)", marginTop: 2 }}>
                Cron 表达式: {status?.checkin_cron}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
