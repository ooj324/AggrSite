import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Account, Site } from '../api';
import { Plus, Edit2, Trash2, Users, RefreshCw, X, Play } from 'lucide-react';
import { format } from 'date-fns';

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

  const handleAction = async (id: number, type: 'checkin' | 'refresh') => {
    setActionLoading(id);
    try {
      if (type === 'checkin') await api.post(`/api/checkin/${id}`);
      if (type === 'refresh') await api.post(`/api/balance/refresh/${id}`);
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
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h2 className="greeting">账户</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={() => openEdit()} className="btn btn-primary">
            <Plus size={18} /> 添加账户
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center p-12">
          <span className="spinner spinner-lg text-primary" />
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>账户</th>
                  <th>站点</th>
                  <th>状态</th>
                  <th>余额 / 已用 / 总额</th>
                  <th>上次操作</th>
                  <th style={{ width: 150, textAlign: 'right' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map(acc => (
                  <tr key={acc.id}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <Users size={16} color="var(--color-text-secondary)" />
                        <span style={{ fontWeight: 500, color: 'var(--color-text-primary)' }}>{acc.username || `Account #${acc.id}`}</span>
                      </div>
                    </td>
                    <td>
                      <span className="badge">{acc.site_name}</span>
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: acc.status === 'active' ? 'var(--color-success)' : 'var(--color-danger)' }} />
                        <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{acc.status === 'active' ? '已启用' : '已禁用'}</span>
                      </div>
                    </td>
                    <td>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                        <span style={{ fontWeight: 500, color: 'var(--color-primary)' }}>${acc.balance?.toFixed(2) || '0.00'}</span>
                        <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>已用: ${acc.balance_used?.toFixed(2) || '0.00'} / 总额: ${acc.quota?.toFixed(2) || '0.00'}</span>
                      </div>
                    </td>
                    <td>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 2, fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        <span>签到: {acc.last_checkin_at ? format(new Date(acc.last_checkin_at), 'MM/dd HH:mm') : '从未'}</span>
                        <span>刷新: {acc.last_balance_refresh ? format(new Date(acc.last_balance_refresh), 'MM/dd HH:mm') : '从未'}</span>
                      </div>
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 4 }}>
                        <button onClick={() => handleAction(acc.id, 'checkin')} disabled={actionLoading === acc.id} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-primary)' }} title="签到">
                          <Play size={16} className={actionLoading === acc.id ? 'animate-pulse' : ''} />
                        </button>
                        <button onClick={() => handleAction(acc.id, 'refresh')} disabled={actionLoading === acc.id} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-info)' }} title="刷新余额">
                          <RefreshCw size={16} className={actionLoading === acc.id ? 'animate-spin' : ''} />
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
                {accounts.length === 0 && (
                  <tr>
                    <td colSpan={6} style={{ textAlign: 'center', padding: 48, color: 'var(--color-text-secondary)' }}>
                      未找到账户。添加一个以开始。
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

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
    status: account?.status || 'active',
    checkin_enabled: account?.checkin_enabled ?? true,
  });
  const [loading, setLoading] = useState(false);

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
        if (account) {
          await api.put(`/api/accounts/${account.id}`, formData);
        } else {
          await api.post('/api/accounts', {
            ...formData,
            site_id: Number(formData.site_id)
          });
        }
      }
      onSaved();
    } catch (err: any) {
      alert(`错误: ${err}`);
      setLoading(false);
    }
  };

  return (
    <div className="modal-backdrop" style={{ position: 'fixed', inset: 0, zIndex: 50, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(4px)' }}>
      <div className="card animate-scale-in" style={{ width: '100%', maxWidth: 440, padding: 24, position: 'relative' }}>
        <button onClick={onClose} className="btn btn-ghost" style={{ position: 'absolute', top: 16, right: 16, padding: 6, minWidth: 'auto' }}>
          <X size={20} />
        </button>
        <h2 style={{ fontSize: 20, fontWeight: 600, marginBottom: 24 }}>{account ? '编辑账户' : '添加账户'}</h2>
        
        {!account && (
          <div className="tabs" style={{ marginBottom: 24 }}>
            <button 
              type="button"
              onClick={() => setMode('login')} 
              className={`tab ${mode === 'login' ? 'active' : ''}`}
            >
              登录模式
            </button>
            <button 
              type="button"
              onClick={() => setMode('token')} 
              className={`tab ${mode === 'token' ? 'active' : ''}`}
            >
              令牌模式
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>站点</label>
            <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.site_id} onChange={e => setFormData({...formData, site_id: Number(e.target.value)})}>
              {sites.map((s: Site) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </div>
          
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>用户名 {mode === 'token' && '(可选)'}</label>
            <input required={mode === 'login'} type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.username} onChange={e => setFormData({...formData, username: e.target.value})} />
          </div>

          {mode === 'login' && !account ? (
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>密码</label>
              <input required type="password" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})} />
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 8 }}>密码用于自动刷新令牌。它将被加密存储。</p>
            </div>
          ) : (
            <>
              <div>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Access Token</label>
                <input required type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.access_token} onChange={e => setFormData({...formData, access_token: e.target.value})} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>API Token (可选)</label>
                <input type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.api_token} onChange={e => setFormData({...formData, api_token: e.target.value})} />
              </div>
            </>
          )}

          {mode === 'token' && (
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>状态</label>
              <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                <option value="active">启用</option>
                <option value="disabled">禁用</option>
              </select>
            </div>
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
          <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
            <button type="button" onClick={onClose} className="btn btn-ghost">取消</button>
            <button type="submit" disabled={loading} className="btn btn-primary">
              {loading ? '保存中...' : '保存'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
