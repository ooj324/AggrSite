import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { RefreshCw, Save, Settings as SettingsIcon } from 'lucide-react';

export default function Settings() {
  const [status, setStatus] = useState<SchedulerStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState({
    CHECKIN_CRON: '',
    BALANCE_REFRESH_CRON: '',
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [schedRes, checkinSet, balanceSet] = await Promise.all([
        api.get('/api/scheduler/status'),
        api.get('/api/settings/CHECKIN_CRON').catch(() => ({ data: { value: '' } })),
        api.get('/api/settings/BALANCE_REFRESH_CRON').catch(() => ({ data: { value: '' } }))
      ]);
      
      setStatus(schedRes.data);
      
      const statusData = schedRes.data;
      setFormData({
        CHECKIN_CRON: checkinSet.data?.value || statusData.checkin_cron || '',
        BALANCE_REFRESH_CRON: balanceSet.data?.value || statusData.balance_refresh_cron || '',
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

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.put('/api/settings/CHECKIN_CRON', { value: formData.CHECKIN_CRON });
      await api.put('/api/settings/BALANCE_REFRESH_CRON', { value: formData.BALANCE_REFRESH_CRON });
      alert('Settings saved and scheduler reloaded successfully!');
      loadData();
    } catch (err: any) {
      alert(`Error saving settings: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="flex justify-center p-12"><RefreshCw className="animate-spin text-primary" size={32} /></div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">System Settings</h1>
      </div>

      <div className="glass-panel p-6 max-w-2xl">
        <div className="flex items-center gap-3 mb-6 border-b border-white/5 pb-4">
          <div className="w-10 h-10 rounded-xl bg-accent/20 text-accent flex items-center justify-center">
            <SettingsIcon size={24} />
          </div>
          <div>
            <h2 className="text-xl font-semibold">Scheduler Configuration</h2>
            <p className="text-sm text-textSecondary">Configure cron expressions for background jobs.</p>
          </div>
        </div>

        <form onSubmit={handleSave} className="space-y-6">
          <div>
            <label className="block text-sm font-medium text-textPrimary mb-1">Check-in Cron Expression</label>
            <p className="text-xs text-textSecondary mb-2">Controls how often automated check-ins run. Empty means disabled.</p>
            <input 
              type="text" 
              className="input-field font-mono" 
              placeholder="e.g. 0 8 * * *" 
              value={formData.CHECKIN_CRON} 
              onChange={e => setFormData({...formData, CHECKIN_CRON: e.target.value})} 
            />
            {status?.next_checkin && (
              <p className="text-xs text-success mt-2">Next Run: {new Date(status.next_checkin).toLocaleString()}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-textPrimary mb-1">Balance Refresh Cron Expression</label>
            <p className="text-xs text-textSecondary mb-2">Controls how often account balances are synced. Empty means disabled.</p>
            <input 
              type="text" 
              className="input-field font-mono" 
              placeholder="e.g. 0 * * * *" 
              value={formData.BALANCE_REFRESH_CRON} 
              onChange={e => setFormData({...formData, BALANCE_REFRESH_CRON: e.target.value})} 
            />
            {status?.next_balance_refresh && (
              <p className="text-xs text-success mt-2">Next Run: {new Date(status.next_balance_refresh).toLocaleString()}</p>
            )}
          </div>

          <div className="pt-4 flex justify-end">
            <button type="submit" disabled={saving} className="btn-primary flex items-center gap-2">
              {saving ? <RefreshCw className="animate-spin" size={18} /> : <Save size={18} />}
              {saving ? 'Saving...' : 'Save & Reload'}
            </button>
          </div>
        </form>
      </div>

      <div className="glass-panel p-6 max-w-2xl">
        <div className="flex items-center gap-3 mb-6 border-b border-white/5 pb-4">
          <div className="w-10 h-10 rounded-xl bg-primary/20 text-primary flex items-center justify-center">
            <SettingsIcon size={24} />
          </div>
          <div>
            <h2 className="text-xl font-semibold">Data Management</h2>
            <p className="text-sm text-textSecondary">Export your current database or import from a backup (supports V2 Legacy & AggrSite formats).</p>
          </div>
        </div>

        <div className="space-y-4">
          <div>
            <h3 className="text-md font-medium text-textPrimary">Export Database</h3>
            <p className="text-xs text-textSecondary mb-3">Download a JSON file containing all Sites and Accounts.</p>
            <button 
              onClick={() => {
                const url = api.defaults.baseURL ? api.defaults.baseURL + '/api/backup/export' : '/api/backup/export';
                const a = document.createElement('a');
                a.href = url + '?token=' + localStorage.getItem('AUTH_TOKEN');
                a.download = 'aggrsite-backup.json';
                a.click();
              }}
              className="btn-secondary text-sm"
            >
              Export to JSON
            </button>
          </div>

          <div className="pt-4 border-t border-white/5">
            <h3 className="text-md font-medium text-textPrimary">Import Database</h3>
            <p className="text-xs text-textSecondary mb-3">Upload a JSON backup to restore or migrate data. Duplicates are merged based on URLs/Usernames.</p>
            <div className="flex gap-2 items-center">
              <input 
                type="file" 
                id="backup-upload"
                className="hidden"
                accept=".json"
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  const formData = new FormData();
                  formData.append('file', file);
                  setSaving(true);
                  try {
                    const res = await api.post('/api/backup/import', formData, {
                      headers: { 'Content-Type': 'multipart/form-data' }
                    });
                    alert(`Import successful!\nSites imported: ${res.data.imported_sites}\nAccounts imported: ${res.data.imported_accounts}`);
                    loadData();
                  } catch (err: any) {
                    alert('Import failed: ' + err);
                  } finally {
                    setSaving(false);
                    e.target.value = '';
                  }
                }}
              />
              <button 
                onClick={() => document.getElementById('backup-upload')?.click()}
                disabled={saving}
                className="btn-primary text-sm"
              >
                Select Backup File
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
