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
      <div className="flex justify-center p-12 animate-fade-in">
        <span className="w-10 h-10 border-4 border-primary/20 border-t-primary rounded-full animate-spin-slow" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">仪表盘</h2>
        <div className="flex gap-2">
          <button onClick={handleRunAllCheckins} disabled={actionLoading} className="relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-sm transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none">
            执行签到
          </button>
          <button onClick={handleRefreshAllBalances} disabled={actionLoading} className="relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-primary bg-primaryLight border border-primary/20 rounded-sm transition-all duration-200 hover:bg-primary/10 hover:border-primary/30 hover:text-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none">
            {actionLoading ? '刷新中...' : '刷新余额'}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <div className="bg-surface rounded-xl p-5 border border-border flex flex-col gap-4 shadow-card hover:-translate-y-1 hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.05s' }}>
          <div className="flex items-center gap-2 text-[13px] font-semibold text-textSecondary mb-1">
            <svg className="w-4 h-4 text-primary opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
            </svg>
            系统数据
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-infoSoft text-info dark:bg-[#172554] dark:text-[#60a5fa]">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">站点总数</div>
              <div className="text-lg font-bold tracking-tight text-textPrimary whitespace-nowrap">{stats.sites}</div>
            </div>
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-successSoft text-success dark:bg-[#052e16] dark:text-[#4ade80]">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 0-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">账户总数</div>
              <div className="text-lg font-bold tracking-tight text-textPrimary whitespace-nowrap">{stats.accounts}</div>
            </div>
          </div>
        </div>

        <div className="bg-surface rounded-xl p-5 border border-border flex flex-col gap-4 shadow-card hover:-translate-y-1 hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.1s' }}>
          <div className="flex items-center gap-2 text-[13px] font-semibold text-textSecondary mb-1">
            <svg className="w-4 h-4 text-primary opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            账户数据
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-infoSoft text-info dark:bg-[#172554] dark:text-[#60a5fa]">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">总余额</div>
              <div className="text-[18px] font-bold tracking-tight text-textPrimary whitespace-nowrap">${stats.total_balance?.toFixed(2) || '0.00'}</div>
            </div>
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-successSoft text-success dark:bg-[#052e16] dark:text-[#4ade80]">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">总消耗</div>
              <div className="text-[18px] font-bold tracking-tight text-textPrimary whitespace-nowrap">${stats.total_used?.toFixed(2) || '0.00'}</div>
            </div>
          </div>
        </div>

        <div className="bg-surface rounded-xl p-5 border border-border flex flex-col gap-4 shadow-card hover:-translate-y-1 hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.15s' }}>
          <div className="flex items-center gap-2 text-[13px] font-semibold text-textSecondary mb-1">
            <svg className="w-4 h-4 text-primary opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            签到状态
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-400">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">今日签到</div>
              <div className="text-lg font-bold tracking-tight text-textPrimary whitespace-nowrap">{stats.checkins_success} / {stats.checkins_today}</div>
            </div>
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-orange-100 text-orange-600 dark:bg-orange-900/40 dark:text-orange-400">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">成功率</div>
              <div className="text-lg font-bold tracking-tight text-textPrimary whitespace-nowrap">{Math.round(stats.checkin_rate)}%</div>
            </div>
          </div>
        </div>

        <div className="bg-surface rounded-xl p-5 border border-border flex flex-col gap-4 shadow-card hover:-translate-y-1 hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.2s' }}>
          <div className="flex items-center gap-2 text-[13px] font-semibold text-textSecondary mb-1">
            <svg className="w-4 h-4 text-primary opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            调度器
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className={`flex items-center justify-center shrink-0 w-10 h-10 rounded-lg ${status?.running ? 'bg-successSoft text-success dark:bg-[#052e16] dark:text-[#4ade80]' : 'bg-dangerSoft text-danger dark:bg-[#450a0a] dark:text-[#f87171]'}`}>
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">状态</div>
              <div className="text-[16px] font-bold tracking-tight text-textPrimary whitespace-nowrap">
                {status?.running ? '运行中' : '已停止'}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex items-center justify-center shrink-0 w-10 h-10 rounded-lg bg-yellow-100 text-yellow-600 dark:bg-yellow-900/40 dark:text-yellow-400">
              <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[12px] text-textMuted mb-0.5 whitespace-nowrap">下次签到</div>
              <div className="text-[16px] font-bold tracking-tight text-textPrimary whitespace-nowrap">
                {status?.next_checkin ? format(new Date(status.next_checkin), 'HH:mm') : '无'}
              </div>
              <div className="text-[11px] text-textMuted mt-[2px] truncate" title={status?.checkin_cron}>
                Cron: {status?.checkin_cron}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
