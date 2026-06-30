import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { format } from 'date-fns';
import { useAlert } from '../components/AlertProvider';

export default function Dashboard() {
  const { showAlert } = useAlert();
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
      setStatus(schedRes as any);
      setStats((statsRes as any) || {
        sites: 0, accounts: 0, total_balance: 0, total_used: 0, checkins_today: 0, checkins_success: 0, checkin_rate: 0
      });
    } catch (err: any) {
      console.error(err);
      showAlert(`加载失败: ${err}`);
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
      showAlert('所有签到已成功触发。');
    } catch (err: any) {
      showAlert(`错误: ${err}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleRefreshAllBalances = async () => {
    if (!confirm('确定要现在刷新所有余额吗？')) return;
    setActionLoading(true);
    try {
      await api.post('/api/balance/refresh/all');
      showAlert('所有余额已成功刷新。');
    } catch (err: any) {
      showAlert(`错误: ${err}`);
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
      <div className="flex flex-wrap items-center justify-between gap-3 mb-8">
        <h2 className="text-[24px] font-bold tracking-tight text-textPrimary m-0">仪表盘</h2>
        <div className="flex gap-2">
          <button onClick={handleRunAllCheckins} disabled={actionLoading} className="relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-lg transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none">
            执行签到
          </button>
          <button onClick={handleRefreshAllBalances} disabled={actionLoading} className="relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-primary bg-primaryLight border border-primary/20 rounded-lg transition-all duration-200 hover:bg-primary/10 hover:border-primary/30 hover:text-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none">
            {actionLoading ? '刷新中...' : '刷新余额'}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-8">
        
        {/* Card 1: Total Balance */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.05s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">总余额</div>
            <div className="w-10 h-10 rounded-xl bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
            </div>
          </div>
          <div className="text-[28px] font-bold tracking-tight text-textPrimary">${stats.total_balance?.toFixed(2) || '0.00'}</div>
          <div className="text-[12px] text-textMuted mt-1">系统所有账户的可用余额</div>
        </div>

        {/* Card 2: Total Used */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.1s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">总消耗</div>
            <div className="w-10 h-10 rounded-xl bg-rose-50 text-rose-600 dark:bg-rose-900/30 dark:text-rose-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" /></svg>
            </div>
          </div>
          <div className="text-[28px] font-bold tracking-tight text-textPrimary">${stats.total_used?.toFixed(2) || '0.00'}</div>
          <div className="text-[12px] text-textMuted mt-1">历史总消费金额</div>
        </div>

        {/* Card 3: Accounts */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.15s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">账户总数</div>
            <div className="w-10 h-10 rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 0-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
            </div>
          </div>
          <div className="text-[28px] font-bold tracking-tight text-textPrimary">{stats.accounts}</div>
          <div className="text-[12px] text-textMuted mt-1">纳管的所有子账户数量</div>
        </div>

        {/* Card 4: Sites */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.2s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">接入站点</div>
            <div className="w-10 h-10 rounded-xl bg-indigo-50 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" /></svg>
            </div>
          </div>
          <div className="text-[28px] font-bold tracking-tight text-textPrimary">{stats.sites}</div>
          <div className="text-[12px] text-textMuted mt-1">已配置的中转或官方平台</div>
        </div>

        {/* Card 5: Checkins */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.25s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">今日签到</div>
            <div className="w-10 h-10 rounded-xl bg-purple-50 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
            </div>
          </div>
          <div className="flex items-baseline gap-2">
            <div className="text-[28px] font-bold tracking-tight text-textPrimary">{stats.checkins_success}</div>
            <div className="text-[16px] font-medium text-textMuted">/ {stats.checkins_today}</div>
          </div>
          <div className="text-[12px] text-textMuted mt-1">成功次数 / 运行总次数</div>
        </div>

        {/* Card 6: Checkin Rate */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.3s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">签到成功率</div>
            <div className="w-10 h-10 rounded-xl bg-orange-50 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
            </div>
          </div>
          <div className="text-[28px] font-bold tracking-tight text-textPrimary">{Math.round(stats.checkin_rate)}%</div>
          <div className="text-[12px] text-textMuted mt-1">过去 24 小时内的成功率</div>
        </div>

        {/* Card 7: Scheduler */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.35s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">调度器引擎</div>
            <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${status?.running ? 'bg-teal-50 text-teal-600 dark:bg-teal-900/30 dark:text-teal-400' : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-400'}`}>
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" /></svg>
            </div>
          </div>
          <div className={`text-[24px] font-bold tracking-tight ${status?.running ? 'text-success' : 'text-danger'}`}>
            {status?.running ? 'Running' : 'Stopped'}
          </div>
          <div className="text-[12px] text-textMuted mt-1">后台 Cron 守护进程</div>
        </div>

        {/* Card 8: Next Run */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.4s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">下次签到任务</div>
            <div className="w-10 h-10 rounded-xl bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400 flex items-center justify-center">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
            </div>
          </div>
          <div className="text-[24px] font-bold tracking-tight text-textPrimary">
            {status?.next_checkin ? format(new Date(status.next_checkin), 'HH:mm') : '--:--'}
          </div>
          <div className="text-[12px] text-textMuted mt-1 truncate" title={status?.checkin_cron || '无'}>
            Cron: {status?.checkin_cron || 'Disabled'}
          </div>
        </div>

        {/* Card 9: Sub2API Refresh */}
        <div className="bg-surface rounded-2xl p-5 border border-border shadow-sm hover:shadow-md transition-all duration-300 animate-slide-up" style={{ animationDelay: '0.45s' }}>
          <div className="flex items-center justify-between mb-4">
            <div className="text-[14px] font-medium text-textSecondary">Sub2API 刷新</div>
            <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${status?.sub2api_refresh_running ? 'bg-cyan-50 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400' : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-400'}`}>
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v6h6M20 20v-6h-6M20 9A8 8 0 006.4 4.8L4 10m16 4l-2.4 5.2A8 8 0 014 15" /></svg>
            </div>
          </div>
          <div className={`text-[24px] font-bold tracking-tight ${status?.sub2api_refresh_running ? 'text-success' : 'text-danger'}`}>
            {status?.sub2api_refresh_running ? 'Running' : 'Stopped'}
          </div>
          <div className="text-[12px] text-textMuted mt-1">
            每 {status?.sub2api_refresh_interval_seconds || 300}s 检查，提前 {status?.sub2api_refresh_lead_seconds || 600}s 刷新
          </div>
        </div>

      </div>
    </div>
  );
}
