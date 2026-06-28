import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { RefreshCw, Save, Settings as SettingsIcon } from 'lucide-react';
import { useAlert } from '../components/AlertProvider';

export default function Settings() {
  const { showAlert } = useAlert();
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
        api.get('/api/settings/CHECKIN_CRON').catch(() => ({ value: '' })),
        api.get('/api/settings/BALANCE_REFRESH_CRON').catch(() => ({ value: '' })),
        api.get('/api/settings/system_proxy_url').catch(() => ({ value: '' }))
      ]);
      
      setStatus(schedRes as any);
      const statusData = schedRes as any;
      
      setFormData({
        CHECKIN_CRON: (checkinSet as any)?.value || statusData.checkin_cron || '',
        BALANCE_REFRESH_CRON: (balanceSet as any)?.value || statusData.balance_refresh_cron || '',
        SYSTEM_PROXY_URL: (proxySet as any)?.value || '',
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

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.put('/api/settings/CHECKIN_CRON', { value: formData.CHECKIN_CRON });
      await api.put('/api/settings/BALANCE_REFRESH_CRON', { value: formData.BALANCE_REFRESH_CRON });
      await api.put('/api/settings/system_proxy_url', { value: formData.SYSTEM_PROXY_URL });
      showAlert('设置已成功保存，且调度器重载成功！');
      loadData();
    } catch (err: any) {
      showAlert(`保存设置时出错: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return (
    <div className="flex justify-center p-12">
      <span className="w-10 h-10 border-4 border-primary/20 border-t-primary rounded-full animate-spin-slow" />
    </div>
  );

  const inputClass = "w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-[13px] text-textPrimary placeholder:text-textMuted focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all font-mono";
  const btnClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-sm transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnSecondaryClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-textPrimary bg-surface border border-border rounded-sm transition-all duration-200 hover:bg-surfaceHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">系统设置</h2>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        {/* 系统与调度器配置 */}
        <div className="bg-surface rounded-xl p-6 border border-border shadow-sm">
          <div className="flex items-center gap-3 mb-6 pb-4 border-b border-border">
            <div className="w-10 h-10 rounded-xl bg-primaryLight text-primary flex items-center justify-center shrink-0">
              <SettingsIcon size={20} />
            </div>
            <div>
              <h2 className="text-[18px] font-semibold m-0 text-textPrimary">系统与调度器配置</h2>
              <p className="text-[13px] text-textSecondary m-0">配置全局系统设置和 Cron 定时任务。</p>
            </div>
          </div>

          <form onSubmit={handleSave} className="flex flex-col gap-5">
            <div>
              <label className="block text-[13px] font-medium text-textSecondary mb-1.5">系统代理 URL</label>
              <p className="text-[12px] text-textMuted mb-2">全局 HTTP 代理（例如 http://127.0.0.1:7890）。留空则使用环境变量或禁用。</p>
              <input 
                type="text" 
                className={inputClass}
                placeholder="http://127.0.0.1:7890" 
                value={formData.SYSTEM_PROXY_URL} 
                onChange={e => setFormData({...formData, SYSTEM_PROXY_URL: e.target.value})} 
              />
            </div>

            <div>
              <label className="block text-[13px] font-medium text-textSecondary mb-1.5">签到 Cron 表达式</label>
              <p className="text-[12px] text-textMuted mb-2">控制自动签到的运行频率。留空表示禁用。</p>
              <input 
                type="text" 
                className={inputClass}
                placeholder="例如 0 8 * * *" 
                value={formData.CHECKIN_CRON} 
                onChange={e => setFormData({...formData, CHECKIN_CRON: e.target.value})} 
              />
              {status?.next_checkin && (
                <p className="text-[12px] text-success mt-2">下一次运行: {new Date(status.next_checkin).toLocaleString()}</p>
              )}
            </div>

            <div>
              <label className="block text-[13px] font-medium text-textSecondary mb-1.5">余额刷新 Cron 表达式</label>
              <p className="text-[12px] text-textMuted mb-2">控制同步账户余额的频率。留空表示禁用。</p>
              <input 
                type="text" 
                className={inputClass}
                placeholder="例如 0 * * * *" 
                value={formData.BALANCE_REFRESH_CRON} 
                onChange={e => setFormData({...formData, BALANCE_REFRESH_CRON: e.target.value})} 
              />
              {status?.next_balance_refresh && (
                <p className="text-[12px] text-success mt-2">下一次运行: {new Date(status.next_balance_refresh).toLocaleString()}</p>
              )}
            </div>

            <div className="pt-4 flex justify-end">
              <button type="submit" disabled={saving} className={btnClass}>
                {saving ? <RefreshCw className="animate-spin" size={16} /> : <Save size={16} />}
                {saving ? '保存中...' : '保存并重载'}
              </button>
            </div>
          </form>
        </div>

        {/* 数据管理 */}
        <div className="bg-surface rounded-xl p-6 border border-border shadow-sm">
          <div className="flex items-center gap-3 mb-6 pb-4 border-b border-border">
            <div className="w-10 h-10 rounded-xl bg-infoSoft text-info flex items-center justify-center shrink-0">
              <SettingsIcon size={20} />
            </div>
            <div>
              <h2 className="text-[18px] font-semibold m-0 text-textPrimary">数据管理</h2>
              <p className="text-[13px] text-textSecondary m-0">从备份中导出或导入。</p>
            </div>
          </div>

          <div className="flex flex-col gap-6">
            <div>
              <h3 className="text-[14px] font-medium text-textPrimary m-0">导出数据库</h3>
              <p className="text-[12px] text-textMuted mb-3 mt-1">下载包含所有站点和账户信息的 JSON 文件。</p>
              <button 
                onClick={() => {
                  const url = api.defaults.baseURL ? api.defaults.baseURL + '/api/backup/export' : '/api/backup/export';
                  const a = document.createElement('a');
                  a.href = url + '?token=' + localStorage.getItem('AUTH_TOKEN');
                  a.download = 'aggrsite-backup.json';
                  a.click();
                }}
                className={btnSecondaryClass}
              >
                导出为 JSON
              </button>
            </div>

            <div className="pt-4 border-t border-border">
              <h3 className="text-[14px] font-medium text-textPrimary m-0">导入数据库</h3>
              <p className="text-[12px] text-textMuted mb-3 mt-1">上传 JSON 备份文件以恢复或迁移数据。</p>
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
                      const data = res as any;
                      showAlert(`导入成功！\n已导入站点数: ${data.imported_sites}\n已导入账户数: ${data.imported_accounts}`);
                      loadData();
                    } catch (err: any) {
                      showAlert('导入失败: ' + err);
                    } finally {
                      setSaving(false);
                      e.target.value = '';
                    }
                  }}
                />
                <button 
                  onClick={() => document.getElementById('backup-upload')?.click()}
                  disabled={saving}
                  className={btnClass}
                >
                  选择备份文件
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
