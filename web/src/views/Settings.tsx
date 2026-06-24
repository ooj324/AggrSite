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

  if (loading) return (
    <div className="flex justify-center p-12">
      <span className="spinner spinner-lg text-primary" />
    </div>
  );

  return (
    <div className="animate-fade-in">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h2 className="greeting">System Settings</h2>
      </div>

      <div className="card" style={{ maxWidth: 600, padding: 24, marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24, paddingBottom: 16, borderBottom: '1px solid var(--color-border)' }}>
          <div style={{ width: 40, height: 40, borderRadius: 12, background: 'var(--color-primary-light)', color: 'var(--color-primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <SettingsIcon size={20} />
          </div>
          <div>
            <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>Scheduler Configuration</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>Configure cron expressions for background jobs.</p>
          </div>
        </div>

        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Check-in Cron Expression</label>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 8 }}>Controls how often automated check-ins run. Empty means disabled.</p>
            <input 
              type="text" 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontFamily: 'monospace' }} 
              placeholder="e.g. 0 8 * * *" 
              value={formData.CHECKIN_CRON} 
              onChange={e => setFormData({...formData, CHECKIN_CRON: e.target.value})} 
            />
            {status?.next_checkin && (
              <p style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 8 }}>Next Run: {new Date(status.next_checkin).toLocaleString()}</p>
            )}
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Balance Refresh Cron Expression</label>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 8 }}>Controls how often account balances are synced. Empty means disabled.</p>
            <input 
              type="text" 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontFamily: 'monospace' }} 
              placeholder="e.g. 0 * * * *" 
              value={formData.BALANCE_REFRESH_CRON} 
              onChange={e => setFormData({...formData, BALANCE_REFRESH_CRON: e.target.value})} 
            />
            {status?.next_balance_refresh && (
              <p style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 8 }}>Next Run: {new Date(status.next_balance_refresh).toLocaleString()}</p>
            )}
          </div>

          <div style={{ paddingTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
            <button type="submit" disabled={saving} className="btn btn-primary" style={{ gap: 8 }}>
              {saving ? <RefreshCw className="animate-spin" size={16} /> : <Save size={16} />}
              {saving ? 'Saving...' : 'Save & Reload'}
            </button>
          </div>
        </form>
      </div>

      <div className="card" style={{ maxWidth: 600, padding: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24, paddingBottom: 16, borderBottom: '1px solid var(--color-border)' }}>
          <div style={{ width: 40, height: 40, borderRadius: 12, background: 'var(--color-info-light)', color: 'var(--color-info)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <SettingsIcon size={20} />
          </div>
          <div>
            <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>Data Management</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>Export or import from a backup.</p>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <div>
            <h3 style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>Export Database</h3>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 12 }}>Download a JSON file containing all Sites and Accounts.</p>
            <button 
              onClick={() => {
                const url = api.defaults.baseURL ? api.defaults.baseURL + '/api/backup/export' : '/api/backup/export';
                const a = document.createElement('a');
                a.href = url + '?token=' + localStorage.getItem('AUTH_TOKEN');
                a.download = 'aggrsite-backup.json';
                a.click();
              }}
              className="btn btn-secondary"
            >
              Export to JSON
            </button>
          </div>

          <div style={{ paddingTop: 16, borderTop: '1px solid var(--color-border)' }}>
            <h3 style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>Import Database</h3>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 12 }}>Upload a JSON backup to restore or migrate data.</p>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input 
                type="file" 
                id="backup-upload"
                style={{ display: 'none' }}
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
                className="btn btn-primary"
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
