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
    if (!confirm('Delete this account?')) return;
    try {
      await api.delete(`/api/accounts/${id}`);
      loadData();
    } catch (err: any) {
      alert(`Error: ${err}`);
    }
  };

  const handleAction = async (id: number, type: 'checkin' | 'refresh') => {
    setActionLoading(id);
    try {
      if (type === 'checkin') await api.post(`/api/checkin/${id}`);
      if (type === 'refresh') await api.post(`/api/balance/refresh/${id}`);
      loadData();
    } catch (err: any) {
      alert(`Error: ${err}`);
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
        <h2 className="greeting">Accounts</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={() => openEdit()} className="btn btn-primary">
            <Plus size={18} /> Add Account
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center p-12">
          <span className="spinner spinner-lg text-primary" />
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
          {accounts.map(acc => (
            <div key={acc.id} className="card p-5 group flex flex-col" style={{ position: 'relative' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <div style={{ width: 40, height: 40, borderRadius: 12, background: 'var(--color-success-soft)', color: 'var(--color-success)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Users size={20} />
                  </div>
                  <div>
                    <h3 style={{ fontWeight: 600, fontSize: 16, margin: 0 }}>{acc.username || `Account #${acc.id}`}</h3>
                    <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>{acc.site_name}</span>
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 4 }}>
                  <button onClick={() => handleAction(acc.id, 'checkin')} disabled={actionLoading === acc.id} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-primary)' }} title="Checkin">
                    <Play size={16} className={actionLoading === acc.id ? 'animate-pulse' : ''} />
                  </button>
                  <button onClick={() => handleAction(acc.id, 'refresh')} disabled={actionLoading === acc.id} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-info)' }} title="Refresh Balance">
                    <RefreshCw size={16} className={actionLoading === acc.id ? 'animate-spin' : ''} />
                  </button>
                  <button onClick={() => openEdit(acc)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto' }}>
                    <Edit2 size={16} />
                  </button>
                  <button onClick={() => handleDelete(acc.id)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-danger)' }}>
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, margin: '12px 0', flex: 1 }}>
                  <div style={{ background: 'var(--color-bg)', borderRadius: 'var(--radius-sm)', padding: 12 }}>
                    <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 4 }}>Balance</p>
                    <p style={{ fontSize: 18, fontWeight: 700, margin: 0 }}>${acc.balance?.toFixed(2) || '0.00'}</p>
                  </div>
                  <div style={{ background: 'var(--color-bg)', borderRadius: 'var(--radius-sm)', padding: 12 }}>
                    <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 4 }}>Usage</p>
                    <p style={{ fontSize: 18, fontWeight: 700, margin: 0 }}>${acc.balance_used?.toFixed(2) || '0.00'}</p>
                  </div>
                  <div style={{ background: 'var(--color-bg)', borderRadius: 'var(--radius-sm)', padding: 12 }}>
                    <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 4 }}>Quota</p>
                    <p style={{ fontSize: 18, fontWeight: 700, margin: 0 }}>${acc.quota?.toFixed(2) || '0.00'}</p>
                  </div>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 12 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span>Last Refresh: {acc.last_balance_refresh ? format(new Date(acc.last_balance_refresh), 'MM/dd HH:mm') : 'Never'}</span>
                    <span>Last Checkin: {acc.last_checkin_at ? format(new Date(acc.last_checkin_at), 'MM/dd HH:mm') : 'Never'}</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span>API Token: {acc.api_token ? '✅ Setup' : '❌ Missing'}</span>
                    <span>Auto Checkin: {acc.checkin_enabled ? '✅ On' : '❌ Off'}</span>
                  </div>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 'auto', paddingTop: 12, borderTop: '1px solid var(--color-border)', fontSize: 12, color: 'var(--color-text-secondary)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span style={{ width: 8, height: 8, borderRadius: '50%', background: acc.status === 'active' ? 'var(--color-success)' : 'var(--color-danger)' }} />
                    <span style={{ textTransform: 'capitalize' }}>{acc.status}</span>
                  </div>
                </div>
              </div>
            ))}
            {accounts.length === 0 && (
              <div className="card" style={{ gridColumn: '1 / -1', textAlign: 'center', padding: 48, color: 'var(--color-text-secondary)' }}>
                No accounts found. Add one to get started.
              </div>
            )}
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
          alert('Successfully logged in and fetched API token!');
        } else {
          alert('Successfully logged in, but no active API token found.');
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
      alert(`Error: ${err}`);
      setLoading(false);
    }
  };

  return (
    <div className="modal-backdrop" style={{ position: 'fixed', inset: 0, zIndex: 50, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(4px)' }}>
      <div className="card animate-scale-in" style={{ width: '100%', maxWidth: 440, padding: 24, position: 'relative' }}>
        <button onClick={onClose} className="btn btn-ghost" style={{ position: 'absolute', top: 16, right: 16, padding: 6, minWidth: 'auto' }}>
          <X size={20} />
        </button>
        <h2 style={{ fontSize: 20, fontWeight: 600, marginBottom: 24 }}>{account ? 'Edit Account' : 'Add Account'}</h2>
        
        {!account && (
          <div style={{ display: 'flex', gap: 8, marginBottom: 24, background: 'var(--color-bg)', padding: 4, borderRadius: 'var(--radius-md)' }}>
            <button 
              type="button"
              onClick={() => setMode('login')} 
              style={{ flex: 1, padding: '8px 0', borderRadius: 'var(--radius-sm)', background: mode === 'login' ? 'var(--color-bg-elevated)' : 'transparent', color: mode === 'login' ? 'var(--color-primary)' : 'var(--color-text-secondary)', fontWeight: mode === 'login' ? 600 : 500, border: 'none', cursor: 'pointer', transition: 'all 0.2s' }}
            >
              Login Mode
            </button>
            <button 
              type="button"
              onClick={() => setMode('token')} 
              style={{ flex: 1, padding: '8px 0', borderRadius: 'var(--radius-sm)', background: mode === 'token' ? 'var(--color-bg-elevated)' : 'transparent', color: mode === 'token' ? 'var(--color-primary)' : 'var(--color-text-secondary)', fontWeight: mode === 'token' ? 600 : 500, border: 'none', cursor: 'pointer', transition: 'all 0.2s' }}
            >
              Token Mode
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Site</label>
            <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.site_id} onChange={e => setFormData({...formData, site_id: Number(e.target.value)})}>
              {sites.map((s: Site) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </div>
          
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Username {mode === 'token' && '(optional)'}</label>
            <input required={mode === 'login'} type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.username} onChange={e => setFormData({...formData, username: e.target.value})} />
          </div>

          {mode === 'login' && !account ? (
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Password</label>
              <input required type="password" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})} />
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 8 }}>Password is used to automatically refresh tokens. It will be stored encrypted.</p>
            </div>
          ) : (
            <>
              <div>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Access Token</label>
                <input required type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.access_token} onChange={e => setFormData({...formData, access_token: e.target.value})} />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>API Token (optional)</label>
                <input type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.api_token} onChange={e => setFormData({...formData, api_token: e.target.value})} />
              </div>
            </>
          )}

          {mode === 'token' && (
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>Status</label>
              <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                <option value="active">Active</option>
                <option value="disabled">Disabled</option>
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
            <label htmlFor="checkin_enabled" style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)' }}>Enable Auto Check-in</label>
          </div>
          <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
            <button type="button" onClick={onClose} className="btn btn-ghost">Cancel</button>
            <button type="submit" disabled={loading} className="btn btn-primary">
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
