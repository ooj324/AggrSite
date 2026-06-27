import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Site } from '../api';
import { Plus, Edit2, Trash2 } from 'lucide-react';
import { Modal } from '../components/Modal';

export default function Sites() {
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingSite, setEditingSite] = useState<Site | null>(null);
  const [platforms, setPlatforms] = useState<string[]>([]);
  
  // Selection & Batch
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [batchLoading, setBatchLoading] = useState(false);

  // Sorting
  const [sortBy, setSortBy] = useState<'custom' | 'balance'>('custom');

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
    if (!confirm('您确定要删除此站点吗？关联账号也会被一并删除。')) return;
    try {
      await api.delete(`/api/sites/${id}`);
      setSelectedIds(selectedIds.filter(x => x !== id));
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    }
  };

  const handleToggleStatus = async (site: Site) => {
    const newStatus = site.status === 'active' ? 'disabled' : 'active';
    if (newStatus === 'disabled') {
      if (!confirm(`确定要禁用站点 [${site.name}] 吗？\n所有关联的账号将会被一并禁用！`)) return;
    }
    try {
      await api.put(`/api/sites/${site.id}`, { status: newStatus });
      loadData();
    } catch (err: any) {
      alert(`错误: ${err}`);
    }
  };

  const handleBatchAction = async (action: string) => {
    if (selectedIds.length === 0) return;
    if (action === 'delete') {
      if (!confirm(`确定要删除选中的 ${selectedIds.length} 个站点吗？关联账号也会被一并删除。`)) return;
    } else if (action === 'disable') {
      if (!confirm(`确定要禁用选中的 ${selectedIds.length} 个站点吗？所有关联的账号将会被一并禁用！`)) return;
    }

    setBatchLoading(true);
    try {
      const res = await api.post('/api/sites/batch', { ids: selectedIds, action });
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
      setSelectedIds(sortedSites.map(s => s.id));
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

  const sortedSites = [...sites].sort((a, b) => {
    if (sortBy === 'balance') {
      return (b.total_balance || 0) - (a.total_balance || 0);
    }
    // 'custom' sort -> usually we sort by sort_order but since we don't have it exposed yet, fallback to ID
    return a.id - b.id;
  });

  const openEdit = (site?: Site) => {
    setEditingSite(site || null);
    setShowModal(true);
  };

  return (
    <div className="animate-fade-in">
      <div className="page-header" style={{ flexWrap: 'wrap', gap: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <h2 className="page-title">站点</h2>
          <div className="pill-tabs" style={{ background: 'var(--color-bg)' }}>
            <div className={`pill-tab ${sortBy === 'custom' ? 'active' : ''}`} onClick={() => setSortBy('custom')}>自定义排序</div>
            <div className={`pill-tab ${sortBy === 'balance' ? 'active' : ''}`} onClick={() => setSortBy('balance')}>按余额排序</div>
          </div>
        </div>
        <button onClick={() => openEdit()} className="btn btn-primary">
          <Plus size={16} style={{ marginRight: 6 }} /> 添加站点
        </button>
      </div>

      {selectedIds.length > 0 && (
        <div className="batch-action-bar animate-fade-in">
          <div className="batch-action-count">已选择 {selectedIds.length} 个站点</div>
          <div className="batch-action-buttons">
            <button disabled={batchLoading} onClick={() => handleBatchAction('enable')} className="btn btn-secondary btn-sm">启用</button>
            <button disabled={batchLoading} onClick={() => handleBatchAction('disable')} className="btn btn-secondary btn-sm">禁用</button>
            <div className="batch-action-divider" />
            <button disabled={batchLoading} onClick={() => handleBatchAction('enableSystemProxy')} className="btn btn-secondary btn-sm">开系统代理</button>
            <button disabled={batchLoading} onClick={() => handleBatchAction('disableSystemProxy')} className="btn btn-secondary btn-sm">关系统代理</button>
            <div className="batch-action-divider" />
            <button disabled={batchLoading} onClick={() => handleBatchAction('delete')} className="btn btn-danger btn-sm">删除</button>
          </div>
        </div>
      )}

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
                    <th style={{ width: 40, textAlign: 'center' }}>
                      <input 
                        type="checkbox" 
                        checked={sites.length > 0 && selectedIds.length === sites.length}
                        onChange={(e) => toggleSelectAll(e.target.checked)}
                      />
                    </th>
                    <th>名称</th>
                    <th>签到页面</th>
                    <th>余额</th>
                    <th>状态</th>
                    <th>代理配置</th>
                    <th>权重</th>
                    <th>平台</th>
                    <th className="sites-actions-col" style={{ width: 100, textAlign: 'right' }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedSites.map(site => (
                    <tr key={site.id} className={selectedIds.includes(site.id) ? 'selected-row' : ''}>
                      <td style={{ textAlign: 'center' }}>
                        <input 
                          type="checkbox" 
                          checked={selectedIds.includes(site.id)}
                          onChange={(e) => toggleSelect(site.id, e.target.checked)}
                        />
                      </td>
                      <td style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                        {site.name}
                      </td>
                      <td>
                        {site.external_checkin_url || site.url ? (
                          <a href={site.external_checkin_url || site.url} target="_blank" rel="noopener noreferrer" className="badge-link">
                            <span className="badge badge-muted" style={{ fontSize: 11 }}>
                              {site.external_checkin_url || site.url}
                            </span>
                          </a>
                        ) : (
                          <span className="badge badge-muted" style={{ fontSize: 11 }}>-</span>
                        )}
                      </td>
                      <td style={{ fontFamily: 'monospace', fontWeight: 600 }}>
                        ${(site.total_balance || 0).toFixed(2)}
                      </td>
                      <td>
                        <span 
                          className={`badge ${site.status === 'active' ? 'badge-success' : 'badge-error'}`} 
                          style={{ cursor: 'pointer', transition: 'all 0.2s' }}
                          onClick={() => handleToggleStatus(site)}
                          title="点击切换状态"
                        >
                          {site.status === 'active' ? '已启用' : '已禁用'}
                        </span>
                      </td>
                      <td>
                        <span className={`badge ${site.use_system_proxy ? 'badge-info' : 'badge-muted'}`}>
                          {site.use_system_proxy ? 'ON' : 'OFF'}
                        </span>
                      </td>
                      <td>{site.sort_order || 0}</td>
                      <td>
                        <span className="badge badge-info">{site.platform}</span>
                      </td>
                      <td style={{ fontSize: 12, color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {site.created_at ? new Date(site.created_at).toLocaleDateString() : '-'}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 4 }}>
                          <button onClick={() => openEdit(site)} className="btn btn-ghost btn-icon">
                            <Edit2 size={16} />
                          </button>
                          <button onClick={() => handleDelete(site.id)} className="btn btn-ghost btn-icon" style={{ color: 'var(--color-danger)' }}>
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

  // removed inputStyle since we use form-control class

  return (
    <Modal title={site ? '编辑站点' : '添加站点'} onClose={onClose}>
      <div className="modal-body">
          <form id="site-form" onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div className="responsive-form-grid responsive-form-grid-2">
              <input required type="text" className="form-control" value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} placeholder="名称" />
              <input required type="url" className="form-control" value={formData.url} onChange={e => {
                const url = e.target.value;
                let nextPlatform = formData.platform;
                if (!nextPlatform || nextPlatform === 'anyrouter') {
                  if (url.includes('api.openai.com') || url.includes('oneapi') || url.includes('newapi')) {
                    nextPlatform = 'newapi';
                  } else if (url.includes('sub2api') || url.includes('aiproxy')) {
                    nextPlatform = 'sub2api';
                  } else if (url.includes('donehub') || url.includes('oaifree')) {
                    nextPlatform = 'donehub';
                  } else if (url.includes('veloera')) {
                    nextPlatform = 'veloera';
                  }
                }
                setFormData({...formData, url, platform: nextPlatform});
              }} placeholder="URL (例如: https://api.example.com)" />
              <select className="form-control" value={formData.platform} onChange={e => setFormData({...formData, platform: e.target.value})}>
                <option value="" disabled>选择平台</option>
                {platforms.map((p: string) => <option key={p} value={p}>{p}</option>)}
              </select>
              <select className="form-control" value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                <option value="active">启用状态: 启用</option>
                <option value="disabled">启用状态: 禁用</option>
              </select>
              <input type="url" className="form-control" value={formData.proxy_url} onChange={e => setFormData({...formData, proxy_url: e.target.value})} placeholder="代理 URL (可选, http://127.0.0.1:7890)" />
              <input type="url" className="form-control" value={formData.external_checkin_url} onChange={e => setFormData({...formData, external_checkin_url: e.target.value})} placeholder="外部签到 URL (可选)" />
            </div>
            <textarea 
              className="form-control"
              style={{ minHeight: 60, fontFamily: 'monospace', resize: 'vertical' }} 
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
    </Modal>
  );
}
