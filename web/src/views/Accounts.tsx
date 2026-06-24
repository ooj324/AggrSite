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
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Accounts</h1>
        <button onClick={() => openEdit()} className="btn-primary flex items-center gap-2">
          <Plus size={18} /> Add Account
        </button>
      </div>

      {loading ? (
        <div className="flex justify-center p-12"><RefreshCw className="animate-spin text-primary" size={32} /></div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">
          {accounts.map(acc => (
            <div key={acc.id} className="glass-card p-5 group flex flex-col">
              <div className="flex justify-between items-start mb-4">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-green-500/10 text-green-500 flex items-center justify-center">
                    <Users size={20} />
                  </div>
                  <div>
                    <h3 className="font-semibold text-white">{acc.username || `Account #${acc.id}`}</h3>
                    <span className="text-xs text-textSecondary">{acc.site_name}</span>
                  </div>
                </div>
                <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button onClick={() => handleAction(acc.id, 'checkin')} disabled={actionLoading === acc.id} className="p-1.5 text-textSecondary hover:text-accent bg-white/5 rounded-lg hover:bg-accent/10" title="Checkin">
                    <Play size={16} className={actionLoading === acc.id ? 'animate-pulse' : ''} />
                  </button>
                  <button onClick={() => handleAction(acc.id, 'refresh')} disabled={actionLoading === acc.id} className="p-1.5 text-textSecondary hover:text-blue-500 bg-white/5 rounded-lg hover:bg-blue-500/10" title="Refresh Balance">
                    <RefreshCw size={16} className={actionLoading === acc.id ? 'animate-spin' : ''} />
                  </button>
                  <button onClick={() => openEdit(acc)} className="p-1.5 text-textSecondary hover:text-white bg-white/5 rounded-lg hover:bg-white/10">
                    <Edit2 size={16} />
                  </button>
                  <button onClick={() => handleDelete(acc.id)} className="p-1.5 text-textSecondary hover:text-error bg-white/5 rounded-lg hover:bg-error/10">
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4 my-4 flex-1">
                <div className="bg-black/20 rounded-lg p-3">
                  <p className="text-xs text-textSecondary mb-1">Balance</p>
                  <p className="text-lg font-bold text-white">${acc.balance?.toFixed(2) || '0.00'}</p>
                </div>
                <div className="bg-black/20 rounded-lg p-3">
                  <p className="text-xs text-textSecondary mb-1">Usage</p>
                  <p className="text-lg font-bold text-white">${acc.balance_used?.toFixed(2) || '0.00'}</p>
                </div>
              </div>

              <div className="flex items-center justify-between mt-auto pt-4 border-t border-white/5 text-xs text-textSecondary">
                <div className="flex items-center gap-1">
                  <span className={`w-2 h-2 rounded-full ${acc.status === 'active' ? 'bg-success' : 'bg-error'}`} />
                  <span className="capitalize">{acc.status}</span>
                </div>
                <div>
                  Last Ref: {acc.last_balance_refresh ? format(new Date(acc.last_balance_refresh), 'MM/dd HH:mm') : 'Never'}
                </div>
              </div>
            </div>
          ))}
          {accounts.length === 0 && (
            <div className="col-span-full text-center p-12 text-textSecondary glass-panel">
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
  const [formData, setFormData] = useState({
    site_id: account?.site_id || (sites[0]?.id ?? 0),
    username: account?.username || '',
    access_token: account?.access_token || '',
    status: account?.status || 'active',
    checkin_enabled: account?.checkin_enabled ?? true,
  });
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      if (account) {
        await api.put(`/api/accounts/${account.id}`, formData);
      } else {
        await api.post('/api/accounts', {
          ...formData,
          site_id: Number(formData.site_id)
        });
      }
      onSaved();
    } catch (err: any) {
      alert(`Error: ${err}`);
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="glass-panel w-full max-w-md p-6 relative animate-slide-up">
        <button onClick={onClose} className="absolute top-4 right-4 text-textSecondary hover:text-white">
          <X size={20} />
        </button>
        <h2 className="text-2xl font-bold mb-6">{account ? 'Edit Account' : 'Add Account'}</h2>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Site</label>
            <select className="input-field appearance-none" value={formData.site_id} onChange={e => setFormData({...formData, site_id: Number(e.target.value)})}>
              {sites.map((s: Site) => <option key={s.id} value={s.id} className="bg-surface">{s.name}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Username (optional)</label>
            <input type="text" className="input-field" value={formData.username} onChange={e => setFormData({...formData, username: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Access Token</label>
            <input required type="text" className="input-field" value={formData.access_token} onChange={e => setFormData({...formData, access_token: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Status</label>
            <select className="input-field appearance-none" value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
              <option value="active" className="bg-surface">Active</option>
              <option value="disabled" className="bg-surface">Disabled</option>
            </select>
          </div>
          <div className="flex items-center gap-2 pt-2">
            <input 
              type="checkbox" 
              id="checkin_enabled"
              checked={formData.checkin_enabled} 
              onChange={e => setFormData({...formData, checkin_enabled: e.target.checked})}
              className="w-4 h-4 rounded border-white/10 bg-black/20 text-primary focus:ring-primary focus:ring-offset-0"
            />
            <label htmlFor="checkin_enabled" className="text-sm font-medium text-textPrimary">Enable Auto Check-in</label>
          </div>
          <div className="pt-4 flex justify-end gap-3">
            <button type="button" onClick={onClose} className="btn-secondary">Cancel</button>
            <button type="submit" disabled={loading} className="btn-primary">
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
