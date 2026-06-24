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
    SYSTEM_PROXY_URL: '',
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [schedRes, checkinSet, balanceSet, proxySet] = await Promise.all([
        api.get('/api/scheduler/status'),
        api.get('/api/settings/CHECKIN_CRON').catch(() => ({ data: { value: '' } })),
        api.get('/api/settings/BALANCE_REFRESH_CRON').catch(() => ({ data: { value: '' } })),
        api.get('/api/settings/system_proxy_url').catch(() => ({ data: { value: '' } }))
      ]);
      
      setStatus(schedRes.data);
      
      const statusData = schedRes.data;
      setFormData({
        CHECKIN_CRON: checkinSet.data?.value || statusData.checkin_cron || '',
        BALANCE_REFRESH_CRON: balanceSet.data?.value || statusData.balance_refresh_cron || '',
        SYSTEM_PROXY_URL: proxySet.data?.value || '',
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
      await api.put('/api/settings/system_proxy_url', { value: formData.SYSTEM_PROXY_URL });
      alert('设置已成功保存，且调度器重载成功！');
      loadData();
    } catch (err: any) {
      alert(`保存设置时出错: ${err}`);
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
        <h2 className="greeting">系统设置</h2>
      </div>

      <div className="card" style={{ maxWidth: 600, padding: 24, marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24, paddingBottom: 16, borderBottom: '1px solid var(--color-border)' }}>
          <div style={{ width: 40, height: 40, borderRadius: 12, background: 'var(--color-primary-light)', color: 'var(--color-primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <SettingsIcon size={20} />
          </div>
          <div>
            <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>系统与调度器配置</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>配置全局系统设置和 Cron 定时任务。</p>
          </div>
        </div>

        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>系统代理 URL</label>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 8 }}>全局 HTTP 代理（例如 http://127.0.0.1:7890）。留空则使用环境变量或禁用。</p>
            <input 
              type="text" 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontFamily: 'monospace' }} 
              placeholder="http://127.0.0.1:7890" 
              value={formData.SYSTEM_PROXY_URL} 
              onChange={e => setFormData({...formData, SYSTEM_PROXY_URL: e.target.value})} 
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>签到 Cron 表达式</label>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 8 }}>控制自动签到的运行频率。留空表示禁用。</p>
            <input 
              type="text" 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontFamily: 'monospace' }} 
              placeholder="例如 0 8 * * *" 
              value={formData.CHECKIN_CRON} 
              onChange={e => setFormData({...formData, CHECKIN_CRON: e.target.value})} 
            />
            {status?.next_checkin && (
              <p style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 8 }}>下一次运行: {new Date(status.next_checkin).toLocaleString()}</p>
            )}
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>余额刷新 Cron 表达式</label>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 8 }}>控制同步账户余额的频率。留空表示禁用。</p>
            <input 
              type="text" 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontFamily: 'monospace' }} 
              placeholder="例如 0 * * * *" 
              value={formData.BALANCE_REFRESH_CRON} 
              onChange={e => setFormData({...formData, BALANCE_REFRESH_CRON: e.target.value})} 
            />
            {status?.next_balance_refresh && (
              <p style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 8 }}>下一次运行: {new Date(status.next_balance_refresh).toLocaleString()}</p>
            )}
          </div>

          <div style={{ paddingTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
            <button type="submit" disabled={saving} className="btn btn-primary" style={{ gap: 8 }}>
              {saving ? <RefreshCw className="animate-spin" size={16} /> : <Save size={16} />}
              {saving ? '保存中...' : '保存并重载'}
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
            <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>数据管理</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', margin: 0 }}>从备份中导出或导入。</p>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <div>
            <h3 style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>导出数据库</h3>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 12 }}>下载包含所有站点和账户信息的 JSON 文件。</p>
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
              导出为 JSON
            </button>
          </div>

          <div style={{ paddingTop: 16, borderTop: '1px solid var(--color-border)' }}>
            <h3 style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>导入数据库</h3>
            <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 12 }}>上传 JSON 备份文件以恢复或迁移数据。</p>
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
                    alert(`导入成功！\n已导入站点数: ${res.data.imported_sites}\n已导入账户数: ${res.data.imported_accounts}`);
                    loadData();
                  } catch (err: any) {
                    alert('导入失败: ' + err);
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
                选择备份文件
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
