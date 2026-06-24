import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Site } from '../api';
import { Plus, Edit2, Trash2, Globe, RefreshCw, X } from 'lucide-react';

export default function Sites() {
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingSite, setEditingSite] = useState<Site | null>(null);
  const [platforms, setPlatforms] = useState<string[]>([]);

  const loadData = async () => {
    try {
      const [sitesRes, platRes] = await Promise.all([
        api.get('/api/sites'),
        api.get('/api/platforms')
      ]);
      setSites(sitesRes.data || []);
      setPlatforms(platRes.data || []);
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
    if (!confirm('Are you sure you want to delete this site?')) return;
    try {
      await api.delete(`/api/sites/${id}`);
      loadData();
    } catch (err: any) {
      alert(`Error: ${err}`);
    }
  };

  const openEdit = (site?: Site) => {
    setEditingSite(site || null);
    setShowModal(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Sites</h1>
        <button onClick={() => openEdit()} className="btn-primary flex items-center gap-2">
          <Plus size={18} /> Add Site
        </button>
      </div>

      {loading ? (
        <div className="flex justify-center p-12"><RefreshCw className="animate-spin text-primary" size={32} /></div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {sites.map(site => (
            <div key={site.id} className="glass-card p-5 group">
              <div className="flex justify-between items-start mb-4">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-blue-500/10 text-blue-500 flex items-center justify-center">
                    <Globe size={20} />
                  </div>
                  <div>
                    <h3 className="font-semibold text-white">{site.name}</h3>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-white/10 text-textSecondary uppercase tracking-wider">{site.platform}</span>
                  </div>
                </div>
                <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button onClick={() => openEdit(site)} className="p-1.5 text-textSecondary hover:text-white bg-white/5 rounded-lg hover:bg-white/10">
                    <Edit2 size={16} />
                  </button>
                  <button onClick={() => handleDelete(site.id)} className="p-1.5 text-textSecondary hover:text-error bg-white/5 rounded-lg hover:bg-error/10">
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
              <p className="text-sm text-textSecondary truncate" title={site.url}>{site.url}</p>
              <div className="mt-4 flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${site.status === 'active' ? 'bg-success' : 'bg-error'}`} />
                <span className="text-sm text-textSecondary capitalize">{site.status}</span>
              </div>
            </div>
          ))}
          {sites.length === 0 && (
            <div className="col-span-full text-center p-12 text-textSecondary glass-panel">
              No sites found. Add one to get started.
            </div>
          )}
        </div>
      )}

      {showModal && (
        <SiteModal 
          site={editingSite} 
          platforms={platforms}
          onClose={() => setShowModal(false)} 
          onSaved={() => { setShowModal(false); loadData(); }} 
        />
      )}
    </div>
  );
}

function SiteModal({ site, platforms, onClose, onSaved }: any) {
  const [formData, setFormData] = useState({
    name: site?.name || '',
    url: site?.url || '',
    platform: site?.platform || platforms[0] || 'new-api',
    status: site?.status || 'active',
  });
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      if (site) {
        await api.put(`/api/sites/${site.id}`, formData);
      } else {
        await api.post('/api/sites', formData);
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
        <h2 className="text-2xl font-bold mb-6">{site ? 'Edit Site' : 'Add Site'}</h2>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Name</label>
            <input required type="text" className="input-field" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">URL</label>
            <input required type="url" className="input-field" value={formData.url} onChange={e => setFormData({...formData, url: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Platform</label>
            <select className="input-field appearance-none" value={formData.platform} onChange={e => setFormData({...formData, platform: e.target.value})}>
              {platforms.map((p: string) => <option key={p} value={p} className="bg-surface">{p}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-textSecondary mb-1">Status</label>
            <select className="input-field appearance-none" value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
              <option value="active" className="bg-surface">Active</option>
              <option value="disabled" className="bg-surface">Disabled</option>
            </select>
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
