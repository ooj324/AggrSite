import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Site } from '../api';
import { Plus, Edit2, Trash2, X } from 'lucide-react';

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
      <div className="page-header">
        <h2 className="page-title">站点</h2>
        <button onClick={() => openEdit()} className="btn btn-primary">
          <Plus size={16} style={{ marginRight: 6 }} /> 添加站点
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
            {sites.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>平台</th>
                    <th>URL</th>
                    <th>状态</th>
                    <th style={{ width: 100, textAlign: 'right' }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {sites.map(site => (
                    <tr key={site.id}>
                      <td style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                        {site.name}
                      </td>
                      <td>
                        <span className="badge badge-info">{site.platform}</span>
                      </td>
                      <td>
                        {site.url ? (
                          <a href={site.url} target="_blank" rel="noopener noreferrer" className="badge-link">
                            <span className="badge badge-muted" style={{ fontSize: 11 }}>
                              {site.url}
                            </span>
                          </a>
                        ) : (
                          <span className="badge badge-muted" style={{ fontSize: 11 }}>-</span>
                        )}
                      </td>
                      <td>
                        <span className={`badge ${site.status === 'active' ? 'badge-success' : 'badge-error'}`}>
                          {site.status === 'active' ? '已启用' : '已禁用'}
                        </span>
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 4 }}>
                          <button onClick={() => openEdit(site)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto' }}>
                            <Edit2 size={16} />
                          </button>
                          <button onClick={() => handleDelete(site.id)} className="btn btn-ghost" style={{ padding: 6, minWidth: 'auto', color: 'var(--color-danger)' }}>
                            <Trash2 size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {sites.length === 0 && (
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
                <div className="empty-state-title">暂无站点</div>
                <div className="empty-state-desc">点击右上角“添加站点”按钮创建</div>
              </div>
            )}
          </>
        )}
      </div>

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

  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', background: 'var(--color-bg)', color: 'var(--color-text-primary)', fontSize: 13, outline: 'none' };

  return (
    <div className="modal-backdrop">
      <div className="modal-content animate-scale-in" style={{ width: '100%', maxWidth: 440 }}>
        <div className="modal-header">
          <h2 className="modal-title">{site ? '编辑站点' : '添加站点'}</h2>
          <button type="button" onClick={onClose} className="modal-close-button"><X size={20} /></button>
        </div>
        <div className="modal-body">
          <form id="site-form" onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div className="responsive-form-grid responsive-form-grid-2">
              <input required type="text" style={inputStyle} value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} placeholder="名称" />
              <input required type="url" style={inputStyle} value={formData.url} onChange={e => setFormData({...formData, url: e.target.value})} placeholder="URL (例如: https://api.example.com)" />
              <select style={inputStyle} value={formData.platform} onChange={e => setFormData({...formData, platform: e.target.value})}>
                <option value="" disabled>选择平台</option>
                {platforms.map((p: string) => <option key={p} value={p}>{p}</option>)}
              </select>
              <select style={inputStyle} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                <option value="active">启用状态: 启用</option>
                <option value="disabled">启用状态: 禁用</option>
              </select>
              <input type="url" style={inputStyle} value={formData.proxy_url} onChange={e => setFormData({...formData, proxy_url: e.target.value})} placeholder="代理 URL (可选, http://127.0.0.1:7890)" />
              <input type="url" style={inputStyle} value={formData.external_checkin_url} onChange={e => setFormData({...formData, external_checkin_url: e.target.value})} placeholder="外部签到 URL (可选)" />
            </div>
            <textarea 
              style={{ ...inputStyle, minHeight: 60, fontFamily: 'monospace', resize: 'vertical' }} 
              value={formData.custom_headers} 
              onChange={e => setFormData({...formData, custom_headers: e.target.value})} 
              placeholder='自定义 Header (JSON 格式, 可选)&#10;{"X-My-Header": "value"}' 
            />
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}>
              <input 
                type="checkbox" 
                id="use_system_proxy"
                checked={formData.use_system_proxy} 
                onChange={e => setFormData({...formData, use_system_proxy: e.target.checked})}
              />
              <label htmlFor="use_system_proxy" style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)' }}>使用系统代理</label>
            </div>
          </form>
        </div>
        <div className="modal-footer">
          <button type="button" onClick={onClose} className="btn btn-ghost">取消</button>
          <button type="submit" form="site-form" disabled={loading} className="btn btn-primary">
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
