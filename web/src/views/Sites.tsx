import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Site } from '../api';
import { Plus, Edit2, Trash2, Globe, X } from 'lucide-react';

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
    if (!confirm('您确定要删除此站点吗？')) return;
    try {
      await api.delete(`/api/sites/${id}`);
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    }
  };

  const openEdit = (site?: Site) => {
    setEditingSite(site || null);
    setShowModal(true);
  };

  return (
    <div className="animate-fade-in">
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <h2 className="greeting">站点</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={() => openEdit()} className="btn btn-primary">
            <Plus size={18} /> 添加站点
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center p-12">
          <span className="spinner spinner-lg text-primary" />
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 16 }}>
          {sites.map(site => (
            <div key={site.id} className="card p-5 group" style={{ position: 'relative' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <div style={{ width: 40, height: 40, borderRadius: 12, background: 'var(--color-primary-light)', color: 'var(--color-primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Globe size={20} />
                  </div>
                  <div>
                    <h3 style={{ fontWeight: 600, fontSize: 16, margin: 0 }}>{site.name}</h3>
                    <span style={{ fontSize: 12, padding: '2px 8px', borderRadius: 12, background: 'var(--color-border)', color: 'var(--color-text-secondary)', textTransform: 'uppercase' }}>{site.platform}</span>
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <button onClick={() => openEdit(site)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto' }}>
                    <Edit2 size={16} />
                  </button>
                  <button onClick={() => handleDelete(site.id)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-danger)' }}>
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
              <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={site.url}>{site.url}</p>
              <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', background: site.status === 'active' ? 'var(--color-success)' : 'var(--color-danger)' }} />
                <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{site.status === 'active' ? '已启用' : '已禁用'}</span>
              </div>
            </div>
          ))}
          {sites.length === 0 && (
            <div className="card" style={{ gridColumn: '1 / -1', textAlign: 'center', padding: 48, color: 'var(--color-text-secondary)' }}>
              未找到站点。添加一个以开始。
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
    proxy_url: site?.proxy_url || '',
    use_system_proxy: site?.use_system_proxy || false,
    external_checkin_url: site?.external_checkin_url || '',
    custom_headers: site?.custom_headers || '',
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
        <h2 style={{ fontSize: 20, fontWeight: 600, marginBottom: 24 }}>{site ? '编辑站点' : '添加站点'}</h2>
        
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>名称</label>
            <input required type="text" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>URL</label>
            <input required type="url" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.url} onChange={e => setFormData({...formData, url: e.target.value})} />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>平台</label>
            <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.platform} onChange={e => setFormData({...formData, platform: e.target.value})}>
              {platforms.map((p: string) => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>状态</label>
            <select style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
              <option value="active">启用</option>
              <option value="disabled">禁用</option>
            </select>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>代理 URL (可选)</label>
            <input type="url" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.proxy_url} onChange={e => setFormData({...formData, proxy_url: e.target.value})} placeholder="http://127.0.0.1:7890" />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>外部签到 URL (可选)</label>
            <input type="url" style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)' }} value={formData.external_checkin_url} onChange={e => setFormData({...formData, external_checkin_url: e.target.value})} placeholder="https://..." />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: 6 }}>自定义 Header (JSON 格式)</label>
            <textarea 
              style={{ width: '100%', padding: '8px 12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', minHeight: '60px', fontFamily: 'monospace' }} 
              value={formData.custom_headers} 
              onChange={e => setFormData({...formData, custom_headers: e.target.value})} 
              placeholder='{"X-My-Header": "value"}' 
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}>
            <input 
              type="checkbox" 
              id="use_system_proxy"
              checked={formData.use_system_proxy} 
              onChange={e => setFormData({...formData, use_system_proxy: e.target.checked})}
            />
            <label htmlFor="use_system_proxy" style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)' }}>使用系统代理</label>
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
