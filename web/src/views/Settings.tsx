import { useEffect, useState } from 'react';
import { api } from '../api';
import type { SchedulerStatus } from '../api';
import { getAuthToken } from '../utils';
import { RefreshCw, Save, Settings as SettingsIcon, ShieldCheck, Play } from 'lucide-react';
import { useAlert } from '../components/AlertProvider';

export default function Settings() {
  const { showAlert } = useAlert();
  const [status, setStatus] = useState<SchedulerStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testingTurnstile, setTestingTurnstile] = useState(false);

  const [formData, setFormData] = useState({
    CHECKIN_CRON: '',
    BALANCE_REFRESH_CRON: '',
    SYSTEM_PROXY_URL: '',
    TURNSTILE_PROVIDER: 'yescaptcha',
    TURNSTILE_API_KEY: '',
    TURNSTILE_API_URL: '',
    TURNSTILE_AUTO_SOLVE: 'true',
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [
        schedRes,
        checkinSet,
        balanceSet,
        proxySet,
        tsProvider,
        tsKey,
        tsUrl,
        tsAuto
      ] = await Promise.all([
        api.get('/api/scheduler/status'),
        api.get('/api/settings/checkin_cron').catch(() => ({ value: '' })),
        api.get('/api/settings/balance_refresh_cron').catch(() => ({ value: '' })),
        api.get('/api/settings/system_proxy_url').catch(() => ({ value: '' })),
        api.get('/api/settings/turnstile_solver_provider').catch(() => ({ value: 'yescaptcha' })),
        api.get('/api/settings/turnstile_solver_api_key').catch(() => ({ value: '' })),
        api.get('/api/settings/turnstile_solver_api_url').catch(() => ({ value: '' })),
        api.get('/api/settings/turnstile_auto_solve').catch(() => ({ value: 'true' })),
      ]);
      
      setStatus(schedRes as any);
      const statusData = schedRes as any;
      
      setFormData({
        CHECKIN_CRON: (checkinSet as any)?.value || statusData.checkin_cron || '',
        BALANCE_REFRESH_CRON: (balanceSet as any)?.value || statusData.balance_refresh_cron || '',
        SYSTEM_PROXY_URL: (proxySet as any)?.value || '',
        TURNSTILE_PROVIDER: (tsProvider as any)?.value || 'yescaptcha',
        TURNSTILE_API_KEY: (tsKey as any)?.value || '',
        TURNSTILE_API_URL: (tsUrl as any)?.value || '',
        TURNSTILE_AUTO_SOLVE: (tsAuto as any)?.value ?? 'true',
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
      await Promise.all([
        api.put('/api/settings/checkin_cron', { value: formData.CHECKIN_CRON }),
        api.put('/api/settings/balance_refresh_cron', { value: formData.BALANCE_REFRESH_CRON }),
        api.put('/api/settings/system_proxy_url', { value: formData.SYSTEM_PROXY_URL }),
        api.put('/api/settings/turnstile_solver_provider', { value: formData.TURNSTILE_PROVIDER }),
        api.put('/api/settings/turnstile_solver_api_key', { value: formData.TURNSTILE_API_KEY }),
        api.put('/api/settings/turnstile_solver_api_url', { value: formData.TURNSTILE_API_URL }),
        api.put('/api/settings/turnstile_auto_solve', { value: formData.TURNSTILE_AUTO_SOLVE }),
      ]);
      showAlert('设置已成功保存，且调度器重载成功！');
      loadData();
    } catch (err: any) {
      showAlert(`保存设置时出错: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  const handleTestTurnstile = async () => {
    setTestingTurnstile(true);
    try {
      const res: any = await api.post('/api/settings/turnstile-test', {
        provider: formData.TURNSTILE_PROVIDER,
        api_key: formData.TURNSTILE_API_KEY,
        api_url: formData.TURNSTILE_API_URL,
      });
      const urlInfo = res.tested_url ? `\n测试目标: ${res.tested_url}` : '';
      showAlert(`Turnstile 求解测试成功！\n求解器: ${formData.TURNSTILE_PROVIDER}${urlInfo}\nToken 长度: ${res.token_len} 字符`);
    } catch (err: any) {
      showAlert(`Turnstile 求解测试失败: ${err}`);
    } finally {
      setTestingTurnstile(false);
    }
  };

  if (loading) return (
    <div className="flex justify-center p-12">
      <span className="w-10 h-10 border-4 border-primary/20 border-t-primary rounded-full animate-spin-slow" />
    </div>
  );

  const inputClass = "w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-[13px] text-textPrimary placeholder:text-textMuted focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all font-mono";
  const selectClass = "w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-[13px] text-textPrimary focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all";
  const btnClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-sm transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnSecondaryClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-textPrimary bg-surface border border-border rounded-sm transition-all duration-200 hover:bg-surfaceHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">系统设置</h2>
      </div>

      <form onSubmit={handleSave} className="flex flex-col gap-6">
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

            <div className="flex flex-col gap-5">
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
                <p className="text-[12px] text-textMuted mb-2">控制自动签到的运行频率。留空表示禁用。当前调度时区：{status?.timezone || 'Local'}。</p>
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

              <div className="rounded-lg border border-border bg-background px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-[13px] font-medium text-textSecondary">托管会话 Token 刷新</div>
                    <div className="text-[12px] text-textMuted mt-1">
                      固定后台任务，每 {status?.managed_refresh_interval_seconds || 300} 秒检查一次（sub2api / new-api-v1）。
                    </div>
                  </div>
                  <span className={`shrink-0 rounded-sm px-2 py-1 text-[12px] font-medium ${status?.managed_refresh_running ? 'bg-success/10 text-success' : 'bg-danger/10 text-danger'}`}>
                    {status?.managed_refresh_running ? 'Running' : 'Stopped'}
                  </span>
                </div>
                <div className="mt-3 grid grid-cols-2 gap-3 text-[12px]">
                  <div>
                    <div className="text-textMuted">检查间隔</div>
                    <div className="mt-0.5 font-mono text-textPrimary">{status?.managed_refresh_interval_seconds || 300}s</div>
                  </div>
                  <div>
                    <div className="text-textMuted">提前刷新</div>
                    <div className="mt-0.5 font-mono text-textPrimary">{status?.managed_refresh_lead_seconds || 600}s</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Turnstile 验证码求解器配置 */}
          <div className="bg-surface rounded-xl p-6 border border-border shadow-sm">
            <div className="flex items-center gap-3 mb-6 pb-4 border-b border-border">
              <div className="w-10 h-10 rounded-xl bg-purple-500/10 text-purple-500 flex items-center justify-center shrink-0">
                <ShieldCheck size={20} />
              </div>
              <div>
                <h2 className="text-[18px] font-semibold m-0 text-textPrimary">Turnstile 验证码求解器</h2>
                <p className="text-[13px] text-textSecondary m-0">解决签到过程中的 Cloudflare Turnstile 验证码拦截。</p>
              </div>
            </div>

            <div className="flex flex-col gap-5">
              <div>
                <label className="block text-[13px] font-medium text-textSecondary mb-1.5">求解器服务商 (Provider)</label>
                <select 
                  className={selectClass}
                  value={formData.TURNSTILE_PROVIDER}
                  onChange={e => setFormData({...formData, TURNSTILE_PROVIDER: e.target.value})}
                >
                  <option value="yescaptcha">YesCaptcha (推荐，支持国内直连)</option>
                  <option value="capsolver">CapSolver</option>
                  <option value="2captcha">2Captcha</option>
                  <option value="custom">自定义服务 / 本地部署 (Turnstile-Solver)</option>
                </select>
              </div>

              <div>
                <label className="block text-[13px] font-medium text-textSecondary mb-1.5">ClientKey / API Key</label>
                <p className="text-[12px] text-textMuted mb-2">打码服务商的用户密钥 (ClientKey)。</p>
                <input 
                  type="password" 
                  className={inputClass}
                  placeholder="例如 3a7f8e9c..." 
                  value={formData.TURNSTILE_API_KEY} 
                  onChange={e => setFormData({...formData, TURNSTILE_API_KEY: e.target.value})} 
                />
              </div>

              <div>
                <label className="block text-[13px] font-medium text-textSecondary mb-1.5">自定义 API URL (可选)</label>
                <p className="text-[12px] text-textMuted mb-2">
                  留空使用默认服务地址。自建服务可填写如 <code>http://127.0.0.1:5000/solve</code>。
                </p>
                <input 
                  type="text" 
                  className={inputClass}
                  placeholder={
                    formData.TURNSTILE_PROVIDER === 'capsolver' ? 'https://api.capsolver.com' :
                    formData.TURNSTILE_PROVIDER === '2captcha' ? 'https://api.2captcha.com' :
                    formData.TURNSTILE_PROVIDER === 'custom' ? 'http://127.0.0.1:5000/solve' :
                    'https://api.yescaptcha.com'
                  }
                  value={formData.TURNSTILE_API_URL} 
                  onChange={e => setFormData({...formData, TURNSTILE_API_URL: e.target.value})} 
                />
              </div>

              <div className="flex items-center gap-2">
                <input 
                  type="checkbox" 
                  id="turnstile_auto_solve"
                  className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                  checked={formData.TURNSTILE_AUTO_SOLVE === 'true' || formData.TURNSTILE_AUTO_SOLVE === '1'} 
                  onChange={e => setFormData({...formData, TURNSTILE_AUTO_SOLVE: e.target.checked ? 'true' : 'false'})}
                />
                <label htmlFor="turnstile_auto_solve" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">
                  启用自动求解（遇到 TURNSTILE_REQUIRED 时自动调用打码）
                </label>
              </div>

              <div className="rounded-lg border border-border bg-background p-3 text-[12px] text-textMuted leading-relaxed">
                <strong className="text-textSecondary">使用提示：</strong>
                各站点如需使用 Turnstile 自动求解，请在站点编辑弹窗中填写该站点的 <code>Turnstile SiteKey</code>（例如 <code>0x4AAAAAA...</code>，可从网页源代码或 new-api 站点设置中获取）。
              </div>

              <div className="flex items-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={handleTestTurnstile}
                  disabled={testingTurnstile || (!formData.TURNSTILE_API_KEY && formData.TURNSTILE_PROVIDER !== 'custom')}
                  className={btnSecondaryClass}
                >
                  {testingTurnstile ? <RefreshCw className="animate-spin" size={16} /> : <Play size={16} />}
                  {testingTurnstile ? '正在测试求解...' : '测试打码服务连接'}
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* 底部保存与数据管理 */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
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
                  type="button"
                  onClick={() => {
                    const url = api.defaults.baseURL ? api.defaults.baseURL + '/api/backup/export' : '/api/backup/export';
                    const a = document.createElement('a');
                    a.href = url + '?token=' + encodeURIComponent(getAuthToken());
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
                      const uploadData = new FormData();
                      uploadData.append('file', file);
                      setSaving(true);
                      try {
                        const res = await api.post('/api/backup/import', uploadData, {
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
                    type="button"
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

          {/* 全局保存卡片 */}
          <div className="bg-surface rounded-xl p-6 border border-border shadow-sm flex flex-col justify-between h-full">
            <div>
              <h2 className="text-[18px] font-semibold m-0 text-textPrimary mb-2">应用所有更改</h2>
              <p className="text-[13px] text-textSecondary m-0">
                保存所有系统配置、调度器任务和 Turnstile 求解器设置。调度器将自动重载生效。
              </p>
            </div>
            <div className="pt-6 flex justify-end">
              <button type="submit" disabled={saving} className={btnClass}>
                {saving ? <RefreshCw className="animate-spin" size={16} /> : <Save size={16} />}
                {saving ? '保存中...' : '保存全部设置'}
              </button>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}

