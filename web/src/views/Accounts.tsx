import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Account, Site } from '../api';
import { Plus, Edit2, Trash2, CalendarCheck, Link as LinkIcon } from 'lucide-react';
import { Modal } from '../components/Modal';
import { format } from 'date-fns';

const parseAccountExtraConfig = (account: any): Record<string, any> => {
  try {
    return JSON.parse(account?.extra_config || "{}") || {};
  } catch {
    return {};
  }
};

const resolveAccountCredentialMode = (account: any): 'session' | 'apikey' => {
  if (account?.api_token) return 'apikey';
  if (typeof account?.access_token === 'string' && account.access_token.trim()) return 'session';
  return 'apikey';
};

export default function Accounts() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingAccount, setEditingAccount] = useState<Account | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);

  // Selection & Batch
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [batchLoading, setBatchLoading] = useState(false);

  const loadData = async () => {
    try {
      const [accRes, sitesRes] = await Promise.all([
        api.get('/api/accounts'),
        api.get('/api/sites')
      ]);
      setAccounts(accRes.data || []);
      setSites(sitesRes.data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleDelete = async (id: number) => {
    if (!confirm('确定要删除此账户吗？')) return;
    try {
      await api.delete(`/api/accounts/${id}`);
      setSelectedIds(selectedIds.filter(x => x !== id));
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    }
  };

  const handleBatchAction = async (action: string) => {
    if (selectedIds.length === 0) return;
    if (action === 'delete') {
      if (!confirm(`确定要删除选中的 ${selectedIds.length} 个账号吗？`)) return;
    }

    setBatchLoading(true);
    try {
      const res = await api.post('/api/accounts/batch', { ids: selectedIds, action });
      const data = (res as any).data || res;
      if (data.failedItems && data.failedItems.length > 0) {
        alert(`部分操作失败:\n` + data.failedItems.map((f: any) => `ID ${f.id}: ${f.message}`).join('\n'));
      }
      setSelectedIds([]);
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    } finally {
      setBatchLoading(false);
    }
  };

  const toggleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(accounts.map(s => s.id));
    } else {
      setSelectedIds([]);
    }
  };

  const toggleSelect = (id: number, checked: boolean) => {
    if (checked) {
      setSelectedIds([...selectedIds, id]);
    } else {
      setSelectedIds(selectedIds.filter(x => x !== id));
    }
  };

  const handleAction = async (id: number, type: 'checkin' | 'refresh' | 'toggle-checkin' | 'rebind') => {
    if (type === 'rebind') {
      const token = prompt('请输入新的 Access Token 进行换绑：');
      if (!token) return;

      let platformUserId: number | undefined;
      const pid = prompt('请输入 Platform User ID（如果不需要请留空）：');
      if (pid) {
        platformUserId = parseInt(pid, 10);
      }

      setActionLoading(id);
      try {
        await api.post(`/api/accounts/${id}/rebind-session`, {
          accessToken: token,
          platformUserId: platformUserId || undefined
        });
        alert('换绑成功！');
        loadData();
      } catch (err: any) {
        alert(`换绑失败: ${err}`);
      } finally {
        setActionLoading(null);
      }
      return;
    }

    setActionLoading(id);
    try {
      if (type === 'checkin') await api.post(`/api/checkin/${id}`);
      if (type === 'refresh') await api.post(`/api/balance/refresh/${id}`);
      if (type === 'toggle-checkin') {
        const acc = accounts.find(a => a.id === id);
        if (acc) {
          await api.put(`/api/accounts/${id}`, { checkin_enabled: !acc.checkin_enabled });
        }
      }
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    } finally {
      setActionLoading(null);
    }
  };

  const openEdit = (acc?: Account) => {
    setEditingAccount(acc || null);
    setShowModal(true);
  };

  const btnPrimaryClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-sm transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnSecondaryClass = "relative inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-[12px] font-medium text-textPrimary bg-surface border border-border rounded-sm transition-all duration-200 hover:bg-surfaceHover hover:-translate-y-px hover:shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnDangerClass = "relative inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-[12px] font-medium text-white bg-danger rounded-sm transition-all duration-200 hover:bg-danger/90 hover:-translate-y-px hover:shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">账户</h2>
        <button onClick={() => openEdit()} className={btnPrimaryClass}>
          <Plus size={16} /> 添加账户
        </button>
      </div>

      {selectedIds.length > 0 && (
        <div className="flex items-center justify-between bg-primary/10 border border-primary/20 p-3 rounded-xl mb-4 shadow-sm animate-fade-in">
          <div className="text-[13.5px] font-semibold text-primary flex items-center gap-2">
            已选择 {selectedIds.length} 个账号
          </div>
          <div className="flex items-center gap-2">
            <button disabled={batchLoading} onClick={() => handleBatchAction('enable')} className={btnSecondaryClass}>启用</button>
            <button disabled={batchLoading} onClick={() => handleBatchAction('disable')} className={btnSecondaryClass}>禁用</button>
            <div className="w-[1px] h-4 bg-primary/20 mx-1" />
            <button disabled={batchLoading} onClick={() => handleBatchAction('delete')} className={btnDangerClass}>删除</button>
          </div>
        </div>
      )}

      <div className="bg-surface rounded-xl border border-border shadow-sm overflow-x-auto">
        {loading ? (
          <div className="p-6 flex flex-col gap-3">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="flex gap-4">
                <div className="bg-black/5 dark:bg-white/5 rounded w-[120px] h-4 animate-pulse" />
                <div className="bg-black/5 dark:bg-white/5 rounded w-[80px] h-4 animate-pulse" />
                <div className="bg-black/5 dark:bg-white/5 rounded w-[120px] h-4 animate-pulse" />
                <div className="bg-black/5 dark:bg-white/5 rounded w-[70px] h-4 animate-pulse" />
                <div className="bg-black/5 dark:bg-white/5 rounded flex-1 h-4 animate-pulse" />
              </div>
            ))}
          </div>
        ) : (
          <>
            {accounts.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th className="w-11 text-center">
                      <input type="checkbox" checked={selectedIds.length === accounts.length && accounts.length > 0} onChange={(e) => toggleSelectAll(e.target.checked)} />
                    </th>
                    <th>连接名称</th>
                    <th>站点</th>
                    <th>运行健康状态</th>
                    <th>余额</th>
                    <th>已用</th>
                    <th>签到</th>
                    <th className="text-center w-[200px]">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {accounts.map(acc => (
                    <tr key={acc.id} className={`group animate-slide-up ${selectedIds.includes(acc.id) ? '!bg-primary/5' : ''}`}>
                      <td className="text-center">
                        <input
                          type="checkbox"
                          checked={selectedIds.includes(acc.id)}
                          onChange={(e) => toggleSelect(acc.id, e.target.checked)}
                        />
                      </td>
                      <td className="text-textPrimary">
                        <div className="font-semibold">
                          {acc.username || `Account #${acc.id}`}
                        </div>
                        <div className="flex gap-1 mt-1">
                          <span className={`inline-flex items-center px-1.5 py-0.5 rounded-sm text-[10px] font-medium ${resolveAccountCredentialMode(acc) === "apikey" ? "bg-warningSoft text-warning" : "bg-infoSoft text-info"}`}>
                            {resolveAccountCredentialMode(acc) === "apikey" ? "API Key" : "Session"}
                          </span>
                          {parseAccountExtraConfig(acc)?.proxyUrl && (
                            <span className="inline-flex items-center px-1.5 py-0.5 rounded-sm text-[10px] font-medium bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-400">
                              代理
                            </span>
                          )}
                        </div>
                      </td>
                      <td>
                        <span className="inline-flex items-center px-2 py-0.5 rounded-sm text-[11px] font-medium bg-black/5 text-textSecondary dark:bg-white/5">
                          {acc.site_name || sites.find(s => s.id === acc.site_id)?.name || `Site #${acc.site_id}`}
                        </span>
                        {acc.site_platform && (
                          <div className="mt-1 text-[10px] text-textMuted">
                            {acc.site_platform}
                          </div>
                        )}
                      </td>
                      <td>
                        <div className="flex flex-col gap-1.5 items-start">
                          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium ${acc.status === 'active' ? 'bg-successSoft text-success' : 'bg-dangerSoft text-danger'}`}>
                            {acc.status === 'active' ? '正常' : '禁用'}
                          </span>
                          <button onClick={() => handleAction(acc.id, 'refresh')} disabled={actionLoading === acc.id} className="text-[12px] text-primary hover:text-primaryHover hover:underline disabled:opacity-50 disabled:no-underline transition-colors p-0">
                            {actionLoading === acc.id ? <span className="w-3 h-3 border-2 border-primary/20 border-t-primary rounded-full animate-spin inline-block align-middle" /> : '刷新余额'}
                          </button>
                        </div>
                      </td>
                      <td className="font-mono">
                        <div className="font-semibold text-textPrimary">
                          ${(acc.balance || 0).toFixed(2)}
                        </div>
                      </td>
                      <td className="font-mono text-[12px]">
                        <div>${(acc.balance_used || 0).toFixed(2)}</div>
                      </td>
                      <td>
                        <div className="text-[12px] text-textSecondary leading-relaxed">
                          {acc.last_checkin_at ? (
                            <>
                              <div className="text-success font-medium">签到成功</div>
                              <div>{format(new Date(acc.last_checkin_at), 'yyyy-MM-dd HH:mm')}</div>
                            </>
                          ) : (
                            <div className="text-textMuted">暂无签到记录</div>
                          )}
                        </div>
                      </td>
                      <td className="text-center">
                        <div className="flex items-center justify-center gap-1.5 transition-opacity">
                          <button
                            type="button"
                            className={`inline-flex items-center justify-center gap-1 px-2 py-0.5 text-[11px] font-bold rounded transition-all duration-150 ${acc.checkin_enabled ? "bg-green-100 text-green-700 border border-green-300 hover:bg-green-200" : "bg-gray-100 text-gray-500 border border-gray-200 hover:bg-gray-200 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-700"} disabled:opacity-60 disabled:cursor-not-allowed`}
                            onClick={() => handleAction(acc.id, 'toggle-checkin')}
                            disabled={actionLoading === acc.id}
                            title={acc.checkin_enabled ? '已开启自动签到' : '已关闭自动签到'}
                          >
                            {actionLoading === acc.id ? (
                              <span className="w-2.5 h-2.5 border-2 border-current/20 border-t-current rounded-full animate-spin" />
                            ) : acc.checkin_enabled ? (
                              <span>ON</span>
                            ) : (
                              <span>OFF</span>
                            )}
                          </button>
                          <div className="w-[1px] h-3 bg-border" />
                          <button onClick={() => handleAction(acc.id, 'checkin')} disabled={actionLoading === acc.id} className="p-1.5 text-warning hover:text-warning/80 hover:bg-warning/10 rounded-md transition-colors disabled:opacity-50" title="手动签到">
                            {actionLoading === acc.id ? <span className="w-4 h-4 border-2 border-warning/20 border-t-warning rounded-full animate-spin inline-block align-middle" /> : <CalendarCheck size={16} />}
                          </button>
                          <button onClick={() => handleAction(acc.id, 'rebind')} disabled={actionLoading === acc.id} className="p-1.5 text-primary hover:text-primaryHover hover:bg-primary/10 rounded-md transition-colors disabled:opacity-50" title="换绑">
                            <LinkIcon size={16} />
                          </button>
                          <div className="w-[1px] h-3 bg-border mx-0.5" />
                          <button onClick={() => openEdit(acc)} className="p-1.5 text-textSecondary hover:text-primary hover:bg-primary/10 rounded-md transition-colors" title="编辑">
                            <Edit2 size={16} />
                          </button>
                          <button onClick={() => handleDelete(acc.id)} className="p-1.5 text-textSecondary hover:text-danger hover:bg-danger/10 rounded-md transition-colors" title="删除">
                            <Trash2 size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {accounts.length === 0 && (
              <div className="flex flex-col items-center justify-center p-16 text-center">
                <svg className="w-16 h-16 text-textMuted mb-4 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
                <div className="text-[16px] font-semibold text-textPrimary mb-1">暂无账户</div>
                <div className="text-[13px] text-textSecondary">点击右上角“添加账户”按钮创建</div>
              </div>
            )}
          </>
        )}
      </div>

      {showModal && (
        <AccountModal
          account={editingAccount}
          sites={sites}
          onClose={() => setShowModal(false)}
          onSaved={() => { setShowModal(false); loadData(); }}
        />
      )}
    </div>
  );
}

function AccountModal({ account, sites, onClose, onSaved }: any) {
  const [mode, setMode] = useState<'login' | 'session' | 'apikey'>(account ? (account.extra_config?.credentialMode === 'apikey' ? 'apikey' : 'session') : 'session');
  const [formData, setFormData] = useState({
    site_id: account?.site_id || (sites[0]?.id ?? 0),
    username: account?.username || '',
    password: '',
    access_token: account?.access_token || '',
    api_token: account?.api_token || '',
    platform_user_id: account?.extra_config?.platformUserId || '',
    status: account?.status || 'active',
    checkin_enabled: account?.checkin_enabled ?? true,
    proxy_url: account?.extra_config?.proxyUrl || '',
    use_system_proxy: account?.extra_config?.useSystemProxy || false,
    checkin_credential: account?.extra_config?.checkin_credential || '',
    skip_model_fetch: false,
    refresh_token: account?.extra_config?.sub2apiAuth?.refreshToken || '',
    token_expires_at: account?.extra_config?.sub2apiAuth?.tokenExpiresAt?.toString() || '',
  });
  const [loading, setLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [verifyResult, setVerifyResult] = useState<{ success: boolean; tokenType?: string; needsUserId?: boolean; shieldBlocked?: boolean; message?: string; modelCount?: number; models?: string[] } | null>(null);

  const parsedApiKeys = mode === 'apikey' && formData.access_token ? formData.access_token.split(/[\n, ]+/).map(k => k.trim()).filter(Boolean) : [];
  const isBatchApiKeyInput = mode === 'apikey' && parsedApiKeys.length > 1;
  const currentSite = sites.find((s: Site) => s.id === formData.site_id);
  const isSub2Api = currentSite?.platform === 'sub2api';

  const handleVerify = async () => {
    if (!formData.access_token) {
      alert('请先输入 Token');
      return;
    }
    if (isBatchApiKeyInput) {
      alert(`检测到 ${parsedApiKeys.length} 个 API Key，批量模式会在添加时逐条校验`);
      return;
    }
    setVerifyLoading(true);
    setVerifyResult(null);
    try {
      const res = await api.post('/api/accounts/verify-token', {
        siteId: Number(formData.site_id),
        accessToken: formData.access_token,
        platformUserId: formData.platform_user_id ? Number(formData.platform_user_id) : 0,
        credentialMode: mode,
      });
      
      const result = res.data;
      setVerifyResult(result);
    } catch (err: any) {
      setVerifyResult({ success: false, message: err.toString() });
    } finally {
      setVerifyLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (mode !== 'login' && !account && !isBatchApiKeyInput && !verifyResult?.success && !formData.skip_model_fetch) {
      alert('请先验证 Token 成功后再添加账号');
      return;
    }

    setLoading(true);
    try {
      if (mode === 'login' && !account) {
        // Login mode
        const res = await api.post('/api/accounts/login', {
          site_id: Number(formData.site_id),
          username: formData.username,
          password: formData.password,
        });
        if (res.data.api_token_found) {
          alert('成功登录并获取 API 令牌！');
        } else {
          alert('成功登录，但未找到活跃的 API 令牌。');
        }
      } else {
        // Token mode
        const payload = {
          site_id: Number(formData.site_id),
          username: formData.username,
          access_token: formData.access_token,
          accessTokens: isBatchApiKeyInput ? parsedApiKeys : undefined,
          api_token: formData.api_token,
          checkin_enabled: formData.checkin_enabled,
          status: formData.status,
          platformUserId: formData.platform_user_id ? Number(formData.platform_user_id) : undefined,
          credentialMode: mode,
          proxyUrl: formData.proxy_url,
          useSystemProxy: formData.use_system_proxy,
          checkin_credential: formData.checkin_credential,
          skipModelFetch: formData.skip_model_fetch,
          refreshToken: formData.refresh_token,
          tokenExpiresAt: formData.token_expires_at ? Number(formData.token_expires_at) : undefined,
        };

        if (account) {
          await api.put(`/api/accounts/${account.id}`, payload);
        } else {
          const res = await api.post('/api/accounts', payload);
          if (res.data?.batch) {
             alert(`批量添加完成：成功 ${res.data.createdCount}，失败 ${res.data.failedCount}`);
          } else if (res.data?.queued) {
             // alert(res.data.message || '账号已添加，后台正在同步初始化信息。');
          }
        }
      }
      onSaved();
    } catch (err: any) {
      alert(`错误: ${err}`);
      setLoading(false);
    }
  };

  const inputClass = "w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-[13px] text-textPrimary placeholder:text-textMuted focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all font-mono";

  return (
    <Modal title={account ? '编辑账户' : '添加账户'} onClose={onClose}>
      <div className="p-6">
        {!account && (
          <div className="flex bg-black/5 dark:bg-white/5 p-1 rounded-xl mb-6">
            <button
              type="button"
              onClick={() => { setMode('login'); setVerifyResult(null); }}
              className={`flex-1 py-1.5 text-[13px] font-medium rounded-lg transition-all ${mode === 'login' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
            >
              账号密码
            </button>
            <button
              type="button"
              onClick={() => { setMode('session'); setVerifyResult(null); }}
              className={`flex-1 py-1.5 text-[13px] font-medium rounded-lg transition-all ${mode === 'session' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
            >
              Session 令牌
            </button>
            <button
              type="button"
              onClick={() => { setMode('apikey'); setVerifyResult(null); }}
              className={`flex-1 py-1.5 text-[13px] font-medium rounded-lg transition-all ${mode === 'apikey' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
            >
              API Key
            </button>
          </div>
        )}

        <form id="account-form" onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <select className={inputClass} value={formData.site_id} onChange={e => { setFormData({ ...formData, site_id: Number(e.target.value) }); setVerifyResult(null); }}>
              {sites.map((s: Site) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>

            <input required={mode === 'login'} type="text" className={inputClass} value={formData.username} onChange={e => setFormData({ ...formData, username: e.target.value })} placeholder={`用户名 / 连接名称 ${mode !== 'login' ? '(可选)' : ''}`} />

            {mode === 'login' && !account ? (
              <input required type="password" className={inputClass} value={formData.password} onChange={e => setFormData({ ...formData, password: e.target.value })} placeholder="密码" />
            ) : (
              <>
                {mode === 'apikey' && !account ? (
                  <textarea required className={`${inputClass} min-h-[80px] col-span-1 sm:col-span-2`} value={formData.access_token} onChange={e => { setFormData({ ...formData, access_token: e.target.value }); setVerifyResult(null); }} placeholder="粘贴 API Key (支持换行/逗号批量粘贴)" />
                ) : (
                  <input required type="text" className={inputClass} value={formData.access_token} onChange={e => { setFormData({ ...formData, access_token: e.target.value }); setVerifyResult(null); }} placeholder={mode === 'session' ? "Access Token (Session)" : "API Key"} />
                )}
                
                {mode === 'session' && (
                  <input type="text" className={inputClass} value={formData.api_token} onChange={e => setFormData({ ...formData, api_token: e.target.value })} placeholder="API Token (可选，验证可自动获取)" />
                )}
                
                <input type="number" className={inputClass} value={formData.platform_user_id} onChange={e => { setFormData({ ...formData, platform_user_id: e.target.value }); setVerifyResult(null); }} placeholder="用户 ID (可选，部分站点需要)" />
                {mode === 'session' && isSub2Api && (
                  <>
                    <input type="text" className={inputClass} value={formData.refresh_token} onChange={e => setFormData({ ...formData, refresh_token: e.target.value })} placeholder="Sub2API refresh_token (可选)" />
                    <input type="number" className={inputClass} value={formData.token_expires_at} onChange={e => setFormData({ ...formData, token_expires_at: e.target.value })} placeholder="token_expires_at (可选)" />
                  </>
                )}
                <input type="url" className={inputClass} value={formData.proxy_url} onChange={e => setFormData({ ...formData, proxy_url: e.target.value })} placeholder="代理 URL (可选)" />
                
                {mode === 'session' && (
                  <input type="text" className={inputClass} value={formData.checkin_credential} onChange={e => setFormData({ ...formData, checkin_credential: e.target.value })} placeholder="独立签到凭据 (可选)" />
                )}
              </>
            )}

            {mode !== 'login' && (
              <select className={inputClass} value={formData.status} onChange={e => setFormData({ ...formData, status: e.target.value })}>
                <option value="active">启用状态: 启用</option>
                <option value="disabled">启用状态: 禁用</option>
              </select>
            )}
          </div>
          
          {parsedApiKeys.length > 0 && (
             <div className="text-[12px] text-textMuted mt-[-4px]">已识别 {parsedApiKeys.length} 个 API Key{isBatchApiKeyInput ? '，添加时会逐条创建同站点连接并参与轮询' : ''}</div>
          )}

          {verifyResult && verifyResult.success && verifyResult.tokenType === "apikey" && (
            <div className="p-3 bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-300 rounded-lg text-[13px] border border-blue-200 dark:border-blue-800">
              <div className="font-semibold mb-1">API Key 验证成功</div>
              <div>可用模型: <strong>{verifyResult.modelCount} 个</strong></div>
            </div>
          )}
          {verifyResult && verifyResult.success && verifyResult.tokenType === "session" && (
            <div className="p-3 bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-300 rounded-lg text-[13px] border border-blue-200 dark:border-blue-800">
              <div className="font-semibold mb-1">Session 验证成功</div>
            </div>
          )}
          {verifyResult && !verifyResult.success && verifyResult.needsUserId && (
            <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-300 rounded-lg text-[13px] border border-yellow-200 dark:border-yellow-800">
              <div className="font-semibold">此站点要求用户 ID，请补充后重新验证</div>
            </div>
          )}
          {verifyResult && !verifyResult.success && !verifyResult.needsUserId && (
            <div className="p-3 bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-300 rounded-lg text-[13px] border border-red-200 dark:border-red-800">
              <div className="font-semibold">Token 无效或已过期</div>
              <div className="opacity-80 mt-1">{verifyResult.message}</div>
            </div>
          )}

          {mode === 'login' && !account && (
            <p className="text-[12px] text-textMuted mt-[-8px]">密码用于自动刷新令牌。它将被加密存储。</p>
          )}

          <div className="flex flex-wrap items-center gap-x-6 gap-y-3 mt-2">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="checkin_enabled"
                className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                checked={formData.checkin_enabled}
                onChange={e => setFormData({ ...formData, checkin_enabled: e.target.checked })}
              />
              <label htmlFor="checkin_enabled" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">开启自动签到</label>
            </div>

            {mode !== 'login' && (
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="use_system_proxy"
                  className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                  checked={formData.use_system_proxy}
                  onChange={e => setFormData({ ...formData, use_system_proxy: e.target.checked })}
                />
                <label htmlFor="use_system_proxy" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">使用系统代理</label>
              </div>
            )}
            
            {mode === 'apikey' && (
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="skip_model_fetch"
                  className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                  checked={formData.skip_model_fetch}
                  onChange={e => setFormData({ ...formData, skip_model_fetch: e.target.checked })}
                />
                <label htmlFor="skip_model_fetch" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">跳过模型验证（直接添加 API Key）</label>
              </div>
            )}
          </div>
        </form>
      </div>

      <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-border bg-black/5 dark:bg-white/5 rounded-b-2xl">
        {mode !== 'login' && !account && (
          <button type="button" onClick={handleVerify} disabled={verifyLoading || isBatchApiKeyInput} className="mr-auto px-4 py-2 text-[13px] font-medium text-primary hover:text-primaryHover transition-colors disabled:opacity-50">
            {verifyLoading ? '验证中...' : (isBatchApiKeyInput ? '批量添加时校验' : '验证 Token')}
          </button>
        )}
        <button type="button" onClick={onClose} className="px-4 py-2 text-[13px] font-medium text-textSecondary hover:text-textPrimary transition-colors">取消</button>
        <button type="submit" form="account-form" disabled={loading} className="relative inline-flex items-center justify-center gap-1.5 px-5 py-2 text-[13px] font-medium text-white bg-primary rounded-md transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed">
          {loading ? '保存中...' : (isBatchApiKeyInput ? '批量添加连接' : '保存')}
        </button>
      </div>
    </Modal>
  );
}
