import { useEffect, useState } from 'react';
import { api } from '../api';
import type { CheckinLog, Event } from '../api';
import { RefreshCw } from 'lucide-react';
import { format } from 'date-fns';

export default function Logs() {
  const [logs, setLogs] = useState<CheckinLog[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'checkin' | 'events'>('checkin');
  
  const [statusFilter, setStatusFilter] = useState<'all' | 'success' | 'failed' | 'skipped'>('all');
  const [timeFilter, setTimeFilter] = useState<'all' | 'today' | 'week'>('all');
  const [runningAll, setRunningAll] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      if (activeTab === 'checkin') {
        const res = await api.get('/api/checkin/logs?limit=50');
        setLogs(res.data || []);
      } else {
        const res = await api.get('/api/events?limit=50');
        setEvents(res.data || []);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [activeTab]);

  const handleCheckinAll = async () => {
    if (!confirm('确定要运行所有账号的签到任务吗？')) return;
    setRunningAll(true);
    try {
      await api.post('/api/checkin/all');
      alert('批量签到执行完成');
      loadData();
    } catch (err: any) {
      alert(`签到执行失败: ${err}`);
    } finally {
      setRunningAll(false);
    }
  };

  const filteredLogs = logs.filter(log => {
    if (statusFilter !== 'all' && log.status !== statusFilter) return false;
    
    if (timeFilter !== 'all') {
      const logDate = new Date(log.created_at);
      const now = new Date();
      if (timeFilter === 'today') {
        if (logDate.toDateString() !== now.toDateString()) return false;
      } else if (timeFilter === 'week') {
        const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        if (logDate < weekAgo) return false;
      }
    }
    return true;
  });

  return (
    <div className="animate-fade-in">
      <div className="page-header" style={{ flexWrap: 'wrap', gap: 12 }}>
        <h2 className="page-title">日志与事件</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          {activeTab === 'checkin' && (
            <button onClick={handleCheckinAll} disabled={runningAll} className="btn btn-warning">
              {runningAll ? <span className="spinner spinner-sm" style={{ marginRight: 6 }} /> : <RefreshCw size={16} style={{ marginRight: 6 }} />}
              运行所有签到
            </button>
          )}
          <button onClick={loadData} className="btn btn-soft-primary">
            {loading && !runningAll ? <span className="spinner spinner-sm" style={{ marginRight: 6 }} /> : <RefreshCw size={16} style={{ marginRight: 6 }} />}
            刷新
          </button>
        </div>
      </div>

      <div className="pill-tabs" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 8, flex: 1 }}>
          <button
            onClick={() => setActiveTab('checkin')}
            className={`pill-tab ${activeTab === 'checkin' ? 'active' : ''}`}
          >
            签到日志 <span style={{ fontVariantNumeric: "tabular-nums", opacity: 0.7 }}>{logs.length}</span>
          </button>
          <button
            onClick={() => setActiveTab('events')}
            className={`pill-tab ${activeTab === 'events' ? 'active' : ''}`}
          >
            系统事件 <span style={{ fontVariantNumeric: "tabular-nums", opacity: 0.7 }}>{events.length}</span>
          </button>
        </div>
        
        {activeTab === 'checkin' && (
          <div style={{ display: 'flex', gap: 8 }}>
            <select 
              className="form-select" 
              style={{ fontSize: 13, padding: '4px 28px 4px 10px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--color-border)' }}
              value={timeFilter} 
              onChange={e => setTimeFilter(e.target.value as any)}
            >
              <option value="all">所有时间</option>
              <option value="today">今天</option>
              <option value="week">最近7天</option>
            </select>
            <select 
              className="form-select" 
              style={{ fontSize: 13, padding: '4px 28px 4px 10px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--color-border)' }}
              value={statusFilter} 
              onChange={e => setStatusFilter(e.target.value as any)}
            >
              <option value="all">所有状态</option>
              <option value="success">成功</option>
              <option value="failed">失败</option>
              <option value="skipped">跳过</option>
            </select>
          </div>
        )}
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
        ) : activeTab === 'checkin' ? (
          <>
            {logs.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>账号</th>
                    <th>站点</th>
                    <th>状态</th>
                    <th>分类</th>
                    <th>消息内容</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredLogs.map((log, i) => (
                    <tr key={i}>
                      <td style={{ fontSize: 12, color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {format(new Date(log.created_at), 'MM/dd HH:mm:ss')}
                      </td>
                      <td style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                        {log.account_username || `Account #${log.account_id}`}
                      </td>
                      <td>
                        <span className="badge badge-muted" style={{ fontSize: 11 }}>
                          {log.site_name || '-'}
                        </span>
                      </td>
                      <td>
                        <span className={`badge ${log.status === 'success' ? 'badge-success' : log.status === 'failed' ? 'badge-error' : 'badge-muted'}`}>
                          {log.status === 'success' ? '成功' : log.status === 'failed' ? '失败' : '跳过'}
                        </span>
                      </td>
                      <td>
                        {log.failureReason ? (
                          <span className="badge badge-info" title={log.failureReason.actionHint}>
                            {log.failureReason.title}
                          </span>
                        ) : (
                          <span className="badge badge-muted">-</span>
                        )}
                      </td>
                      <td style={{ maxWidth: 360 }}>
                        <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={log.message}>
                          {log.status === 'failed' ? (
                            <span className="badge badge-error" style={{ marginRight: 6 }}>Error</span>
                          ) : log.status === 'success' && log.reward ? (
                            <span className="badge badge-success" style={{ marginRight: 6 }}>奖励: {log.reward}</span>
                          ) : null}
                          <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)' }}>{log.message}</span>
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {filteredLogs.length === 0 && (
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="empty-state-title">暂无匹配的签到日志</div>
              </div>
            )}
          </>
        ) : (
          <>
            {events.length > 0 && (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>类型</th>
                    <th>级别</th>
                    <th>标题</th>
                    <th>消息</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((ev, i) => (
                    <tr key={i}>
                      <td style={{ fontSize: 12, color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {format(new Date(ev.created_at), 'MM/dd HH:mm:ss')}
                      </td>
                      <td>
                        <span className="badge badge-info">{ev.type}</span>
                      </td>
                      <td>
                        <span className={`badge ${ev.level === 'error' ? 'badge-error' : ev.level === 'warning' ? 'badge-warning' : 'badge-success'}`}>
                          {ev.level}
                        </span>
                      </td>
                      <td style={{ fontWeight: 500, color: 'var(--color-text-primary)' }}>
                        {ev.title}
                      </td>
                      <td style={{ maxWidth: 400 }}>
                        <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: 'var(--color-text-secondary)' }} title={ev.message}>
                          {ev.message}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {events.length === 0 && (
              <div className="empty-state">
                <svg className="empty-state-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="empty-state-title">暂无系统事件</div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
