import { useEffect, useState } from 'react';
import { api, detectSite, pingSite } from '../api';
import type { Site } from '../api';
import { Plus, Edit2, Trash2, Activity } from 'lucide-react';
import { Modal } from '../components/Modal';
import { useAlert } from '../components/AlertProvider';

export default function Sites() {
  const { showAlert } = useAlert();
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
      setSites((sitesRes as any) || []);
      setPlatforms((platRes as any) || []);
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

  const handleDelete = async (id: number) => {
    if (!confirm('您确定要删除此站点吗？关联账号也会被一并删除。')) return;
    try {
      await api.delete(`/api/sites/${id}`);
      setSelectedIds(selectedIds.filter(x => x !== id));
      loadData();
    } catch (err: any) {
      showAlert(`错误: ${err}`);
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
      showAlert(`错误: ${err}`);
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
      const data = res as any;
      if (data.failedItems && data.failedItems.length > 0) {
        showAlert(`部分操作失败:\n` + data.failedItems.map((f: any) => `ID ${f.id}: ${f.message}`).join('\n'));
      }
      setSelectedIds([]);
      loadData();
    } catch (err: any) {
      showAlert(`错误: ${err}`);
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

  const handleListPing = async (url: string) => {
    try {
      const response: any = await pingSite(url);
      showAlert(`连通成功! 延迟: ${response.latency_ms}ms (HTTP ${response.status_code})`);
    } catch (err: any) {
      showAlert(`请求失败: ${err}`);
    }
  };

  const btnPrimaryClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-primary rounded-sm transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnSecondaryClass = "relative inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-[12px] font-medium text-textPrimary bg-surface border border-border rounded-sm transition-all duration-200 hover:bg-surfaceHover hover:-translate-y-px hover:shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnDangerClass = "relative inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-[12px] font-medium text-white bg-danger rounded-sm transition-all duration-200 hover:bg-danger/90 hover:-translate-y-px hover:shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <div className="flex items-center gap-4">
          <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">站点</h2>
          <div className="inline-flex gap-0.5 bg-black/5 dark:bg-white/5 rounded-xl p-1">
            <button
              onClick={() => setSortBy('custom')}
              className={`px-3 py-1 text-[12px] font-medium rounded-lg transition-all whitespace-nowrap ${sortBy === 'custom' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
            >
              自定义排序
            </button>
            <button
              onClick={() => setSortBy('balance')}
              className={`px-3 py-1 text-[12px] font-medium rounded-lg transition-all whitespace-nowrap ${sortBy === 'balance' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
            >
              按余额排序
            </button>
          </div>
        </div>
        <button onClick={() => openEdit()} className={btnPrimaryClass}>
          <Plus size={16} /> 添加站点
        </button>
      </div>

      {selectedIds.length > 0 && (
        <div className="flex items-center justify-between bg-primary/10 border border-primary/20 p-3 rounded-xl mb-4 shadow-sm animate-fade-in">
          <div className="text-[13.5px] font-semibold text-primary flex items-center gap-2">
            已选择 {selectedIds.length} 个站点
          </div>
          <div className="flex items-center gap-2">
            <button disabled={batchLoading} onClick={() => handleBatchAction('enable')} className={btnSecondaryClass}>启用</button>
            <button disabled={batchLoading} onClick={() => handleBatchAction('disable')} className={btnSecondaryClass}>禁用</button>
            <div className="w-[1px] h-4 bg-primary/20 mx-1" />
            <button disabled={batchLoading} onClick={() => handleBatchAction('enableSystemProxy')} className={btnSecondaryClass}>开系统代理</button>
            <button disabled={batchLoading} onClick={() => handleBatchAction('disableSystemProxy')} className={btnSecondaryClass}>关系统代理</button>
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
            {sites.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th className="w-10 text-center">
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
                    <th>创建时间</th>
                    <th className="text-center w-[120px]">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedSites.map(site => (
                    <tr key={site.id} className={selectedIds.includes(site.id) ? '!bg-primary/5' : ''}>
                      <td className="text-center">
                        <input 
                          type="checkbox" 
                          checked={selectedIds.includes(site.id)}
                          onChange={(e) => toggleSelect(site.id, e.target.checked)}
                        />
                      </td>
                      <td className="text-textPrimary">
                        <div className="font-semibold">{site.name}</div>
                        <div className="text-[11px] text-textMuted mt-0.5">{site.url}</div>
                      </td>
                      <td>
                        {site.external_checkin_url || site.url ? (
                          <a href={site.external_checkin_url || site.url} target="_blank" rel="noopener noreferrer" className="hover:opacity-80 transition-opacity">
                            <span className="inline-flex items-center px-2 py-0.5 rounded-sm text-[11px] font-medium bg-black/5 text-textSecondary dark:bg-white/5">
                              {site.external_checkin_url || site.url}
                            </span>
                          </a>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-sm text-[11px] font-medium bg-black/5 text-textSecondary dark:bg-white/5">-</span>
                        )}
                      </td>
                      <td className="font-mono font-semibold">
                        ${(site.total_balance || 0).toFixed(2)}
                      </td>
                      <td>
                        <span 
                          className={`inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium cursor-pointer transition-all hover:opacity-80 ${site.status === 'active' ? 'bg-successSoft text-success' : 'bg-dangerSoft text-danger'}`}
                          onClick={() => handleToggleStatus(site)}
                          title="点击切换状态"
                        >
                          {site.status === 'active' ? '已启用' : '已禁用'}
                        </span>
                      </td>
                      <td>
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium ${site.use_system_proxy ? 'bg-infoSoft text-info' : 'bg-black/5 text-textSecondary dark:bg-white/5'}`}>
                          {site.use_system_proxy ? 'ON' : 'OFF'}
                        </span>
                      </td>
                      <td>{site.sort_order || 0}</td>
                      <td>
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium bg-infoSoft text-info">{site.platform}</span>
                      </td>
                      <td className="text-[12px] text-textSecondary whitespace-nowrap">
                        {site.created_at ? new Date(site.created_at).toLocaleDateString() : '-'}
                      </td>
                      <td className="text-center">
                        <div className="flex items-center justify-center gap-1 transition-opacity">
                          <button onClick={() => handleListPing(site.url)} className="p-1.5 text-textSecondary hover:text-info hover:bg-info/10 rounded-md transition-colors" title="测试连通性">
                            <Activity size={16} />
                          </button>
                          <button onClick={() => openEdit(site)} className="p-1.5 text-textSecondary hover:text-primary hover:bg-primary/10 rounded-md transition-colors" title="编辑">
                            <Edit2 size={16} />
                          </button>
                          <button onClick={() => handleDelete(site.id)} className="p-1.5 text-textSecondary hover:text-danger hover:bg-danger/10 rounded-md transition-colors" title="删除">
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
              <div className="flex flex-col items-center justify-center p-16 text-center">
                <svg className="w-16 h-16 text-textMuted mb-4 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
                <div className="text-[16px] font-semibold text-textPrimary mb-1">暂无站点</div>
                <div className="text-[13px] text-textSecondary">点击右上角“添加站点”按钮创建</div>
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
  const { showAlert } = useAlert();
  const [formData, setFormData] = useState({
    name: site?.name || '',
    url: site?.url || '',
    platform: site?.platform || platforms[0] || 'new-api',
    status: site?.status || 'active',
    proxy_url: site?.proxy_url || '',
    use_system_proxy: site?.use_system_proxy || false,
    external_checkin_url: site?.external_checkin_url || '',
    external_checkin_method: site?.external_checkin_method || '',
    external_checkin_auth_header: site?.external_checkin_auth_header || '',
    external_checkin_auth_prefix: site?.external_checkin_auth_prefix ?? '',
    custom_headers: site?.custom_headers || '',
  });

  const [useAdvancedCheckin, setUseAdvancedCheckin] = useState(
    !!site?.external_checkin_method || !!site?.external_checkin_auth_header
  );
  const [advCheckin, setAdvCheckin] = useState({
    method: site?.external_checkin_method || 'POST',
    url: site?.external_checkin_url || '',
    auth_header: site?.external_checkin_auth_header ?? 'Authorization',
    auth_prefix: site?.external_checkin_auth_prefix ?? 'Bearer ',
    body: site?.external_checkin_body ?? ''
  });

  useEffect(() => {
    // Legacy string parsing fallback (if migration from JSON string is needed)
    if (site?.external_checkin_url && !site?.external_checkin_method) {
      const val = site.external_checkin_url.trim();
      if (val.startsWith('{')) {
        try {
          const parsed = JSON.parse(val);
          setAdvCheckin({
            method: parsed.method || 'POST',
            url: parsed.url || '',
            auth_header: parsed.auth_header ?? 'Authorization',
            auth_prefix: parsed.auth_prefix ?? 'Bearer ',
            body: parsed.body ?? ''
          });
          setUseAdvancedCheckin(true);
        } catch (e) {}
      } else if (val.indexOf('|') > 0) {
        const idx = val.indexOf('|');
        setAdvCheckin({
          method: val.substring(0, idx).toUpperCase(),
          url: val.substring(idx + 1),
          auth_header: 'Authorization',
          auth_prefix: 'Bearer ',
          body: ''
        });
        setUseAdvancedCheckin(true);
      }
    }
  }, [site]);

  const [loading, setLoading] = useState(false);
  const [detecting, setDetecting] = useState(false);
  const [pinging, setPinging] = useState(false);

  const handleDetect = async () => {
    if (!formData.url.trim()) {
      showAlert('请先输入 URL');
      return;
    }
    setDetecting(true);
    try {
      const response: any = await detectSite(formData.url);
      setFormData(prev => ({ ...prev, platform: response.platform, url: response.url || prev.url }));
      showAlert(`已识别为平台: ${response.platform}`);
    } catch (err: any) {
      showAlert(`检测失败: ${err}`);
    } finally {
      setDetecting(false);
    }
  };

  const handlePing = async () => {
    if (!formData.url.trim()) {
      showAlert('请先输入 URL');
      return;
    }
    setPinging(true);
    try {
      const response: any = await pingSite(formData.url);
      showAlert(`连通成功! 延迟: ${response.latency_ms}ms (HTTP ${response.status_code})`);
    } catch (err: any) {
      showAlert(`请求失败: ${err}`);
    } finally {
      setPinging(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const submitData: any = { ...formData };
      if (useAdvancedCheckin) {
        if (advCheckin.url && advCheckin.url.trim() !== '') {
          submitData.external_checkin_url = advCheckin.url.trim();
          submitData.external_checkin_method = advCheckin.method;
          submitData.external_checkin_auth_header = advCheckin.auth_header;
          submitData.external_checkin_auth_prefix = advCheckin.auth_prefix;
          const bodyText = advCheckin.body.trim();
          if (bodyText !== '' && bodyText.toLowerCase() !== 'none') {
            try {
              JSON.parse(bodyText);
            } catch (e) {
              showAlert('签到请求体格式错误，必须是有效的 JSON 格式；如需不发送请求体请填 none');
              setLoading(false);
              return;
            }
          }
          submitData.external_checkin_body = advCheckin.body;
        } else {
          showAlert('请填写签到目标 URL，否则无法保存高级签到配置。如果想使用原站点默认URL，请将原URL复制到此处。');
          setLoading(false);
          return;
        }
      } else {
        submitData.external_checkin_method = '';
        submitData.external_checkin_auth_header = '';
        submitData.external_checkin_auth_prefix = '';
        submitData.external_checkin_body = '';
      }

      if (submitData.custom_headers && submitData.custom_headers.trim() !== '') {
        try {
          JSON.parse(submitData.custom_headers);
        } catch (e) {
          showAlert('自定义 Header 格式错误，必须是有效的 JSON 格式');
          setLoading(false);
          return;
        }
      } else {
        submitData.custom_headers = '{}';
      }

      console.log('Submitting data to backend:', submitData);
      if (site) {
        await api.put(`/api/sites/${site.id}`, submitData);
      } else {
        await api.post('/api/sites', submitData);
      }
      onSaved();
    } catch (err: any) {
      showAlert(`错误: ${err}`);
      setLoading(false);
    }
  };

  const inputClass = "w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-[13px] text-textPrimary placeholder:text-textMuted focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-all font-mono";
  const fieldClass = "flex flex-col gap-1.5";
  const labelClass = "text-[12px] font-medium text-textSecondary";

  return (
    <Modal title={site ? '编辑站点' : '添加站点'} onClose={onClose}>
      <div className="p-6">
          <form id="site-form" onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <label className={fieldClass}>
                <span className={labelClass}>站点名称</span>
                <input required type="text" className={inputClass} value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} placeholder="名称" />
              </label>
              <div className={fieldClass}>
                <span className={labelClass}>站点 URL</span>
                <div className="flex gap-2">
                  <input required type="url" className={`${inputClass} flex-1`} value={formData.url} onChange={e => {
                    const url = e.target.value;
                    const lowerUrl = url.toLowerCase();
                    let nextPlatform = formData.platform;
                    if (lowerUrl.includes('agentrouter')) {
                      nextPlatform = 'agentrouter';
                    } else if (!nextPlatform || nextPlatform === 'anyrouter') {
                      if (lowerUrl.includes('api.openai.com') || lowerUrl.includes('oneapi') || lowerUrl.includes('newapi')) {
                        nextPlatform = 'newapi';
                      } else if (lowerUrl.includes('sub2api') || lowerUrl.includes('aiproxy')) {
                        nextPlatform = 'sub2api';
                      } else if (lowerUrl.includes('donehub') || lowerUrl.includes('oaifree')) {
                        nextPlatform = 'donehub';
                      } else if (lowerUrl.includes('veloera')) {
                        nextPlatform = 'veloera';
                      }
                    }
                    setFormData({...formData, url, platform: nextPlatform});
                  }} placeholder="例如: https://api.example.com" />
                  <button type="button" onClick={handleDetect} disabled={detecting || !formData.url.trim()} className="px-3 py-1 bg-surface border border-border rounded-lg text-textPrimary text-[12px] hover:bg-surfaceHover disabled:opacity-50 transition-colors whitespace-nowrap">
                    {detecting ? '检测中' : '自动识别'}
                  </button>
                  <button type="button" onClick={handlePing} disabled={pinging || !formData.url.trim()} className="px-3 py-1 bg-surface border border-border rounded-lg text-textPrimary text-[12px] hover:bg-surfaceHover disabled:opacity-50 transition-colors whitespace-nowrap">
                    {pinging ? 'Ping...' : 'Ping'}
                  </button>
                </div>
              </div>
              <label className={fieldClass}>
                <span className={labelClass}>平台</span>
                <select className={inputClass} value={formData.platform} onChange={e => setFormData({...formData, platform: e.target.value})}>
                  <option value="" disabled>选择平台</option>
                  {platforms.map((p: string) => <option key={p} value={p}>{p}</option>)}
                </select>
              </label>
              <label className={fieldClass}>
                <span className={labelClass}>状态</span>
                <select className={inputClass} value={formData.status} onChange={e => setFormData({...formData, status: e.target.value})}>
                  <option value="active">启用状态: 启用</option>
                  <option value="disabled">启用状态: 禁用</option>
                </select>
              </label>
              <label className={fieldClass}>
                <span className={labelClass}>代理 URL</span>
                <input type="url" className={inputClass} value={formData.proxy_url} onChange={e => setFormData({...formData, proxy_url: e.target.value})} placeholder="仅填 HTTP 代理，例如 http://127.0.0.1:7890；不是签到接口" />
              </label>
              
              {!useAdvancedCheckin && (
                <label className={fieldClass}>
                  <span className={labelClass}>外部签到 URL</span>
                  <input type="url" className={inputClass} value={formData.external_checkin_url} onChange={e => setFormData({...formData, external_checkin_url: e.target.value})} placeholder="普通模式签到接口，可选" />
                </label>
              )}
            </div>

            <div className="flex items-center gap-2">
              <input 
                type="checkbox" 
                id="use_advanced_checkin"
                className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                checked={useAdvancedCheckin} 
                onChange={e => {
                  setUseAdvancedCheckin(e.target.checked);
                  if (e.target.checked && !advCheckin.url) {
                    setAdvCheckin(prev => ({ ...prev, url: formData.external_checkin_url || formData.url }));
                  }
                }}
              />
              <label htmlFor="use_advanced_checkin" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">高级签到配置 (自定义请求方法与认证头)</label>
            </div>

            {useAdvancedCheckin && (
              <div className="bg-black/5 dark:bg-white/5 p-4 rounded-xl flex flex-col gap-3 border border-border/50">
                <div className="flex gap-3">
                  <label className={`${fieldClass} w-[100px] flex-shrink-0`}>
                    <span className={labelClass}>方法</span>
                    <select className={inputClass} value={advCheckin.method} onChange={e => setAdvCheckin({...advCheckin, method: e.target.value})}>
                      <option value="POST">POST</option>
                      <option value="GET">GET</option>
                      <option value="PUT">PUT</option>
                      <option value="PATCH">PATCH</option>
                    </select>
                  </label>
                  <label className={`${fieldClass} flex-1`}>
                    <span className={labelClass}>签到目标 URL</span>
                    <input required={useAdvancedCheckin} type="url" className={inputClass} value={advCheckin.url} onChange={e => setAdvCheckin({...advCheckin, url: e.target.value})} placeholder="实际请求的签到接口，必填" />
                  </label>
                </div>
                <div className="flex gap-3">
                  <label className={`${fieldClass} flex-1`}>
                    <span className={labelClass}>认证 Header 名称</span>
                    <input type="text" className={inputClass} value={advCheckin.auth_header} onChange={e => setAdvCheckin({...advCheckin, auth_header: e.target.value})} placeholder='留空默认 Authorization，输入 none 禁用' />
                  </label>
                  <label className={`${fieldClass} flex-1`}>
                    <span className={labelClass}>认证前缀</span>
                    <input type="text" className={inputClass} value={advCheckin.auth_prefix} onChange={e => setAdvCheckin({...advCheckin, auth_prefix: e.target.value})} placeholder='例如: "Bearer "，Cookie 可填 auth_token=' />
                  </label>
                </div>
                <div className="text-[11px] text-textMuted leading-relaxed">
                  提示：认证信息将通过设置 <code>{advCheckin.auth_header || 'Authorization'}: {advCheckin.auth_header ? (advCheckin.auth_prefix || '') : 'Bearer '}[账号签到凭据]</code> 发送。留空将默认使用 <code>Authorization: Bearer</code> 认证。若无需发送认证，请将 Header 名称设为 <code>none</code>。独立签到凭据请在账号设置中配置。
                </div>
                <textarea
                  className={inputClass}
                  style={{ minHeight: 60, resize: 'vertical' }}
                  value={advCheckin.body}
                  onChange={e => setAdvCheckin({...advCheckin, body: e.target.value})}
                  placeholder='请求体 Body (JSON 格式, 可选)&#10;留空默认发送 {}，填 none 则不发送请求体'
                />
                <div className="text-[11px] text-textMuted leading-relaxed">
                  提示：POST/PUT/PATCH 请求若留空，将默认发送 <code>{'{}'}</code>。如果接口要求空请求体，请填 <code>none</code>。
                </div>
              </div>
            )}

            <textarea 
              className={inputClass}
              style={{ minHeight: 60, resize: 'vertical' }} 
              value={formData.custom_headers} 
              onChange={e => setFormData({...formData, custom_headers: e.target.value})} 
              placeholder='自定义 Header (JSON 格式, 可选)&#10;{"X-My-Header": "value"}' 
            />
            <div className="flex items-center gap-2 mt-1">
              <input 
                type="checkbox" 
                id="use_system_proxy"
                className="w-4 h-4 text-primary bg-background border-border rounded focus:ring-primary focus:ring-2"
                checked={formData.use_system_proxy} 
                onChange={e => setFormData({...formData, use_system_proxy: e.target.checked})}
              />
              <label htmlFor="use_system_proxy" className="text-[13px] font-medium text-textPrimary cursor-pointer select-none">使用系统代理</label>
            </div>
          </form>
        </div>
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-border bg-black/5 dark:bg-white/5 rounded-b-2xl">
          <button type="button" onClick={onClose} className="px-4 py-2 text-[13px] font-medium text-textSecondary hover:text-textPrimary transition-colors">取消</button>
          <button type="submit" form="site-form" disabled={loading} className="relative inline-flex items-center justify-center gap-1.5 px-5 py-2 text-[13px] font-medium text-white bg-primary rounded-md transition-all duration-200 hover:bg-primaryHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed">
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
    </Modal>
  );
}
