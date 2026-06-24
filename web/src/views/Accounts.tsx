import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Account, Site } from '../api';
import { Plus, Edit2, Trash2, X } from 'lucide-react';
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
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    }
  };

  const handleAction = async (id: number, type: 'checkin' | 'refresh' | 'toggle-checkin') => {
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

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <h2 className="page-title">账户</h2>
        <button onClick={() => openEdit()} className="btn btn-primary">
          <Plus size={16} style={{ marginRight: 6 }} /> 添加账户
        </button>
      </div>

      <div className="card" style={{ padding: 0, overflowX: 'auto', borderTopLeftRadius: 0, borderTopRightRadius: 0 }}>
        {loading ? (
          <div style={{ padding: 24, display: "flex", flexDirection: "column", gap: 10 }}>
            {[...Array(5)].map((_, i) => (
              <div key={i} style={{ display: "flex", gap: 16 }}>
                <div className="skeleton" style={{ width: 120, height: 16 }} />
                <div className="skeleton" style={{ width: 80, height: 16 }} />
                <div className="skeleton" style={{ width: 120, height: 16 }} />
                <div className="skeleton" style={{ width: 70, height: 16 }} />
                <div className="skeleton" style={{ flex: 1, height: 16 }} />
              </div>
            ))}
          </div>
        ) : (
          <>
            {accounts.length > 0 && (
              <table className="data-table accounts-table">
                <thead>
                  <tr>
                    <th>连接名称</th>
                    <th>站点</th>
                    <th>运行健康状态</th>
                    <th>余额</th>
                    <th>已用</th>
                    <th>签到信息</th>
                    <th style={{ textAlign: 'right' }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {accounts.map(acc => (
                    <tr key={acc.id} className="animate-slide-up">
                      <td style={{ color: "var(--color-text-primary)" }}>
                        <div style={{ fontWeight: 600 }}>
                          {acc.username || `Account #${acc.id}`}
                        </div>
                        <div style={{ display: "flex", gap: 4, marginTop: 4 }}>
                          <span
                            className={`badge ${resolveAccountCredentialMode(acc) === "apikey" ? "badge-warning" : "badge-info"}`}
                            style={{ fontSize: 10 }}
                          >
                            {resolveAccountCredentialMode(acc) === "apikey"
                              ? "API Key"
                              : "Session"}
                          </span>
                          {parseAccountExtraConfig(acc)?.proxyUrl && (
                            <span
                              className="badge badge-purple"
                              style={{ fontSize: 10 }}
                            >
                              代理
                            </span>
                          )}
                        </div>
                      </td>
                      <td>
                        <span className="badge badge-muted">{acc.site_name || sites.find(s => s.id === acc.site_id)?.name || `Site #${acc.site_id}`}</span>
                      </td>
                      <td>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'flex-start' }}>
                          <span className={`badge ${acc.status === 'active' ? 'badge-success' : 'badge-error'}`}>
                            {acc.status === 'active' ? '正常' : '禁用'}
                          </span>
                          <button onClick={() => handleAction(acc.id, 'refresh')} disabled={actionLoading === acc.id} className="btn btn-link btn-link-primary" style={{ padding: 0, fontSize: 12 }}>
                            {actionLoading === acc.id ? <span className="spinner spinner-sm" /> : '刷新余额'}
                          </button>
                        </div>
                      </td>
                      <td style={{ fontVariantNumeric: "tabular-nums" }}>
                        <div style={{ fontWeight: 600, color: "var(--color-text-primary)" }}>
                          ${(acc.balance || 0).toFixed(2)}
                        </div>
                      </td>
                      <td style={{ fontVariantNumeric: "tabular-nums", fontSize: 12 }}>
                        <div>${(acc.balance_used || 0).toFixed(2)}</div>
                      </td>
                      <td>
                        <div style={{ fontSize: 12, color: "var(--color-text-secondary)", lineHeight: 1.5 }}>
                          {acc.last_checkin_at ? (
                            <>
                              <div style={{ color: 'var(--color-success)', fontWeight: 500 }}>签到成功</div>
                              <div>{format(new Date(acc.last_checkin_at), 'yyyy-MM-dd HH:mm')}</div>
                            </>
                          ) : (
                            <div style={{ color: 'var(--color-text-muted)' }}>暂无签到记录</div>
                          )}
                        </div>
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 8 }}>
                          <button
                            type="button"
                            className={`checkin-toggle-badge ${acc.checkin_enabled ? "is-on" : "is-off"}`}
                            onClick={() => handleAction(acc.id, 'toggle-checkin')}
                            disabled={actionLoading === acc.id}
                            style={{ transform: 'scale(0.85)', transformOrigin: 'right center' }}
                            title={acc.checkin_enabled ? '已开启自动签到' : '已关闭自动签到'}
                          >
                            {actionLoading === acc.id ? (
                              <span className="spinner spinner-sm" />
                            ) : acc.checkin_enabled ? (
                              <span className="status-label">自动签到ON</span>
                            ) : (
                              <span className="status-label">自动签到OFF</span>
                            )}
                          </button>
                          <button onClick={() => handleAction(acc.id, 'checkin')} disabled={actionLoading === acc.id} className="btn btn-link btn-link-warning" style={{ padding: '0 4px' }}>
                            {actionLoading === acc.id ? <span className="spinner spinner-sm" /> : '手动签到'}
                          </button>
                          <button onClick={() => openEdit(acc)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto' }}>
                            <Edit2 size={16} />
                          </button>
                          <button onClick={() => handleDelete(acc.id)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-danger)' }}>
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
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
                <div className="empty-state-title">暂无账户</div>
                <div className="empty-state-desc">点击右上角“添加账户”按钮创建</div>
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
  const [mode, setMode] = useState<'login' | 'token'>(account ? 'token' : 'login');
  const [formData, setFormData] = useState({
    site_id: account?.site_id || (sites[0]?.id ?? 0),
    username: account?.username || '',
    password: '',
    access_token: account?.access_token || '',
    api_token: account?.api_token || '',
    platform_user_id: account?.extra_config?.platformUserId || '',
    status: account?.status || 'active',
    checkin_enabled: account?.checkin_enabled ?? true,
    credential_mode: account?.extra_config?.credentialMode || 'session',
  });
  const [loading, setLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);

  const handleVerify = async () => {
    if (!formData.access_token) {
      alert('请先输入 Access Token');
      return;
    }
    setVerifyLoading(true);
    try {
      const res = await api.post('/api/accounts/verify-token', {
        siteId: Number(formData.site_id),
        accessToken: formData.access_token,
        platformUserId: formData.platform_user_id ? Number(formData.platform_user_id) : 0
      });
      if (res.data.tokenType === 'session') {
        setFormData(prev => ({ 
          ...prev, 
          username: prev.username || res.data.userInfo?.username || '',
          api_token: prev.api_token || res.data.apiToken || '',
          credential_mode: 'session'
        }));
      } else if (res.data.tokenType === 'apikey') {
        setFormData(prev => ({ 
          ...prev, 
          credential_mode: 'apikey'
        }));
      }
      alert(`验证成功！类型: ${res.data.tokenType}`);
    } catch (err: any) {
      alert(`验证失败: ${err}`);
    } finally {
      setVerifyLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      if (mode === 'login' && !account) {
        // Login mode (creating new or updating existing by username)
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
          api_token: formData.api_token,
          checkin_enabled: formData.checkin_enabled,
          status: formData.status,
          platformUserId: formData.platform_user_id ? Number(formData.platform_user_id) : undefined,
          credentialMode: formData.credential_mode
        };
        
        if (account) {
          await api.put(`/api/accounts/${account.id}`, payload);
        } else {
          await api.post('/api/accounts', payload);
        }
      }
      onSaved();
    } catch (err: any) {
      alert(`错误: ${err}`);
      setLoading(false);
    }
  };

  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontSize: 13, outline: 'none' };

  return (
    <div className="modal-backdrop">
      <div className="modal-content animate-scale-in" style={{ width: '100%', maxWidth: 440 }}>
        <div className="modal-header">
          <h2 className="modal-title">{account ? '编辑账户' : '添加账户'}</h2>
          <button type="button" onClick={onClose} className="modal-close-button"><X size={20} /></button>
        </div>
        
        <div className="modal-body">
          {!account && (
            <div className="pill-tabs" style={{ marginBottom: 24, justifyContent: 'center' }}>
              <button 
                type="button"
                onClick={() => setMode('login')} 
                className={`pill-tab ${mode === 'login' ? 'active' : ''}`}
                style={{ flex: 1 }}
              >
                登录模式
              </button>
              <button 
                type="button"
                onClick={() => setMode('token')} 
                className={`pill-tab ${mode === 'token' ? 'active' : ''}`}
                style={{ flex: 1 }}
              >
                令牌模式
              </button>
            </div>
          )}

          <form id="account-form" onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div className="responsive-form-grid responsive-form-grid-2">
              <select style={inputStyle} value={formData.site_id} onChange={e => setFormData({...formData, site_id: Number(e.target.value)})}>
                {sites.map((s: Site) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
              
              <input required={mode === 'login'} type="text" style={inputStyle} value={formData.username} onChange={e => setFormData({...formData, username: e.target.value})} placeholder={`用户名 ${mode === 'token' ? '(可选)' : ''}`} />

              {mode === 'login' && !account ? (
                <input required type="password" style={inputStyle} value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})} placeholder="密码" />
              ) : (
                <>
                  <input required type="text" style={inputStyle} value={formData.access_token} onChange={e => setFormData({...formData, access_token: e.target.value})} placeholder="Access Token 或 API Key" />
                  <input type="text" style={inputStyle} value={formData.api_token} onChange={e => setFormData({...formData, api_token: e.target.value})} placeholder="API Token (可选，验证可自动获取)" />
                  <input type="number" style={inputStyle} value={formData.platform_user_id} onChange={e => setFormData({...formData, platform_user_id: e.target.value})} placeholder="Platform User ID (部分站点需要)" />
                  <select style={inputStyle} value={formData.credential_mode} onChange={e => setFormData({...formData, credential_mode: e.target.value})}>
                    <option value="session">模式: Session (支持签到)</option>
                    <option value="apikey">模式: API Key (仅代理)</option>
                  </select>
                </>
              )}

              {mode === 'token' && (
                <select style={inputStyle} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                  <option value="active">启用状态: 启用</option>
                  <option value="disabled">启用状态: 禁用</option>
                </select>
              )}
            </div>

            {mode === 'login' && !account && (
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: -8 }}>密码用于自动刷新令牌。它将被加密存储。</p>
            )}
            
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}>
              <input 
                type="checkbox" 
                id="checkin_enabled"
                checked={formData.checkin_enabled} 
                onChange={e => setFormData({...formData, checkin_enabled: e.target.checked})}
              />
              <label htmlFor="checkin_enabled" style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)' }}>启用自动签到</label>
            </div>
          </form>
        </div>
        
        <div className="modal-footer">
          {mode === 'token' && !account && (
            <button type="button" onClick={handleVerify} disabled={verifyLoading} className="btn btn-ghost" style={{ marginRight: 'auto', color: 'var(--color-primary)' }}>
              {verifyLoading ? '验证中...' : '验证 Token'}
            </button>
          )}
          <button type="button" onClick={onClose} className="btn btn-ghost">取消</button>
          <button type="submit" form="account-form" disabled={loading} className="btn btn-primary">
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
