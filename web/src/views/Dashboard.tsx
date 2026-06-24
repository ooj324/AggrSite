import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { Play, RefreshCw, Clock, Activity, Users, Database } from 'lucide-react';
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

  if (loading) return <div className="flex justify-center p-12"><RefreshCw className="animate-spin text-primary" size={32} /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <div className="flex gap-3">
          <button onClick={handleRunAllCheckins} disabled={actionLoading} className="btn-primary flex items-center gap-2 bg-accent hover:bg-accent/80 shadow-accent/20">
            <Play size={18} />
            Run Checkins
          </button>
          <button onClick={handleRefreshAllBalances} disabled={actionLoading} className="btn-primary flex items-center gap-2">
            <RefreshCw size={18} className={actionLoading ? 'animate-spin' : ''} />
            Refresh Balances
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="Total Sites" value={stats.sites} icon={Database} color="text-blue-500" bg="bg-blue-500/10" />
        <StatCard title="Total Accounts" value={stats.accounts} icon={Users} color="text-green-500" bg="bg-green-500/10" />
        <StatCard 
          title="Next Checkin" 
          value={status?.next_checkin ? format(new Date(status.next_checkin), 'HH:mm') : 'None'} 
          subtitle={status?.checkin_cron}
          icon={Clock} color="text-accent" bg="bg-accent/10" 
        />
        <StatCard 
          title="Scheduler" 
          value={status?.running ? 'Running' : 'Stopped'} 
          icon={Activity} 
          color={status?.running ? "text-success" : "text-error"} 
          bg={status?.running ? "bg-success/10" : "bg-error/10"} 
        />
      </div>

      <div className="glass-panel p-6">
        <h2 className="text-xl font-semibold mb-4">System Overview</h2>
        <p className="text-textSecondary">
          AggrSite is operating normally. You have {stats.accounts} active accounts across {stats.sites} platforms.
          The automated check-in and balance refresh jobs are scheduled.
        </p>
      </div>
    </div>
  );
}

function StatCard({ title, value, subtitle, icon: Icon, color, bg }: any) {
  return (
    <div className="glass-card p-6 flex items-center gap-4">
      <div className={`w-14 h-14 rounded-2xl flex items-center justify-center ${bg} ${color}`}>
        <Icon size={28} />
      </div>
      <div>
        <p className="text-sm font-medium text-textSecondary">{title}</p>
        <p className="text-2xl font-bold text-white">{value}</p>
        {subtitle && <p className="text-xs text-textSecondary mt-1">{subtitle}</p>}
      </div>
    </div>
  );
}
