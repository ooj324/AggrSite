import { useEffect, useState } from 'react';
import { api } from '../api';
import type { CheckinLog, Event } from '../api';
import { RefreshCw } from 'lucide-react';
import { format } from 'date-fns';
import { useAlert } from '../components/AlertProvider';

type TimeFilter = 'today' | 'week' | 'all';
type PaginatedResult<T> = {
  items: T[];
  total: number;
  limit: number;
  offset: number;
};

const pageSizeOptions = [25, 50, 100];

const getTimeParams = (range: TimeFilter) => {
  if (range === 'all') return {};
  const now = new Date();
  const end = new Date(now);
  const start = new Date(now);
  if (range === 'today') {
    start.setHours(0, 0, 0, 0);
    end.setHours(24, 0, 0, 0);
  } else {
    start.setDate(start.getDate() - 7);
  }
  return {
    start: start.toISOString(),
    end: end.toISOString(),
  };
};

const buildQuery = (params: Record<string, string | number | undefined>) => {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === '' || value === 'all') return;
    query.set(key, String(value));
  });
  return query.toString();
};

export default function Logs() {
  const { showAlert } = useAlert();
  const [logs, setLogs] = useState<CheckinLog[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [eventTotal, setEventTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'checkin' | 'events'>('checkin');

  const [statusFilter, setStatusFilter] = useState<'all' | 'success' | 'failed' | 'skipped'>('all');
  const [eventLevelFilter, setEventLevelFilter] = useState<'all' | 'info' | 'warning' | 'error'>('all');
  const [timeFilter, setTimeFilter] = useState<TimeFilter>('today');
  const [pageSize, setPageSize] = useState(50);
  const [checkinPage, setCheckinPage] = useState(1);
  const [eventPage, setEventPage] = useState(1);
  const [runningAll, setRunningAll] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const timeParams = getTimeParams(timeFilter);
      const checkinQuery = buildQuery({
        limit: pageSize,
        offset: (checkinPage - 1) * pageSize,
        status: statusFilter,
        ...timeParams,
      });
      const eventQuery = buildQuery({
        limit: pageSize,
        offset: (eventPage - 1) * pageSize,
        level: eventLevelFilter,
        ...timeParams,
      });
      const [logsRes, eventsRes] = await Promise.all([
        api.get(`/api/checkin/logs?${checkinQuery}`),
        api.get(`/api/events?${eventQuery}`)
      ]);
      const logPage = logsRes as unknown as PaginatedResult<CheckinLog>;
      const eventPageResult = eventsRes as unknown as PaginatedResult<Event>;
      setLogs(logPage.items || []);
      setLogTotal(logPage.total || 0);
      setEvents(eventPageResult.items || []);
      setEventTotal(eventPageResult.total || 0);
    } catch (err: any) {
      console.error(err);
      showAlert(`加载失败: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [timeFilter, statusFilter, eventLevelFilter, pageSize, checkinPage, eventPage]);

  const handleCheckinAll = async () => {
    if (!confirm('确定要运行所有账号的签到任务吗？')) return;
    setRunningAll(true);
    try {
      await api.post('/api/checkin/all');
      showAlert('批量签到执行完成');
      loadData();
    } catch (err: any) {
      showAlert(`签到执行失败: ${err}`);
    } finally {
      setRunningAll(false);
    }
  };

  const activeTotal = activeTab === 'checkin' ? logTotal : eventTotal;
  const activePage = activeTab === 'checkin' ? checkinPage : eventPage;
  const totalPages = Math.max(1, Math.ceil(activeTotal / pageSize));
  const setActivePage = (page: number) => {
    const next = Math.min(Math.max(1, page), totalPages);
    if (activeTab === 'checkin') setCheckinPage(next);
    else setEventPage(next);
  };

  const selectClass = "px-3 py-1.5 bg-surface border border-border rounded-lg text-[13px] text-textPrimary focus:outline-none focus:border-primary transition-all pr-8";
  const btnWarningClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-white bg-warning rounded-sm transition-all duration-200 hover:bg-warning/90 hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";
  const btnSecondaryClass = "relative inline-flex items-center justify-center gap-1.5 px-4 py-2 text-[13px] font-medium text-textPrimary bg-surface border border-border rounded-sm transition-all duration-200 hover:bg-surfaceHover hover:-translate-y-px hover:shadow-md active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed";

  return (
    <div className="animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h2 className="text-[22px] font-bold tracking-tight text-textPrimary m-0">日志与事件</h2>
        <div className="flex items-center gap-2">
          {activeTab === 'checkin' && (
            <button onClick={handleCheckinAll} disabled={runningAll} className={btnWarningClass}>
              {runningAll ? <span className="w-4 h-4 border-2 border-white/20 border-t-white rounded-full animate-spin" /> : <RefreshCw size={16} />}
              运行所有签到
            </button>
          )}
          <button onClick={loadData} disabled={loading || runningAll} className={btnSecondaryClass}>
            {(loading && !runningAll) ? <span className="w-4 h-4 border-2 border-primary/20 border-t-primary rounded-full animate-spin" /> : <RefreshCw size={16} />}
            刷新
          </button>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-4">
        <div className="inline-flex gap-1 p-1 bg-black/5 dark:bg-white/5 rounded-xl self-start">
          <button
            onClick={() => setActiveTab('checkin')}
            className={`px-4 py-1.5 text-[13px] font-medium rounded-lg transition-all whitespace-nowrap ${activeTab === 'checkin' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
          >
            签到日志 <span className="font-mono opacity-70 ml-1">{logTotal}</span>
          </button>
          <button
            onClick={() => setActiveTab('events')}
            className={`px-4 py-1.5 text-[13px] font-medium rounded-lg transition-all whitespace-nowrap ${activeTab === 'events' ? 'bg-surface text-primary shadow-sm font-semibold' : 'text-textMuted hover:text-textPrimary bg-transparent'}`}
          >
            系统事件 <span className="font-mono opacity-70 ml-1">{eventTotal}</span>
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <select
            className={selectClass}
            value={timeFilter}
            onChange={e => {
              setTimeFilter(e.target.value as TimeFilter);
              setCheckinPage(1);
              setEventPage(1);
            }}
          >
            <option value="today">今天</option>
            <option value="week">最近7天</option>
            <option value="all">所有时间</option>
          </select>
          {activeTab === 'checkin' ? (
            <select 
              className={selectClass}
              value={statusFilter} 
              onChange={e => { setStatusFilter(e.target.value as any); setCheckinPage(1); }}
            >
              <option value="all">所有状态</option>
              <option value="success">成功</option>
              <option value="failed">失败</option>
              <option value="skipped">跳过</option>
            </select>
          ) : (
            <select
              className={selectClass}
              value={eventLevelFilter}
              onChange={e => { setEventLevelFilter(e.target.value as any); setEventPage(1); }}
            >
              <option value="all">所有级别</option>
              <option value="info">信息</option>
              <option value="warning">警告</option>
              <option value="error">错误</option>
            </select>
          )}
          <select
            className={selectClass}
            value={pageSize}
            onChange={e => {
              setPageSize(Number(e.target.value));
              setCheckinPage(1);
              setEventPage(1);
            }}
          >
            {pageSizeOptions.map(size => <option key={size} value={size}>每页 {size}</option>)}
          </select>
        </div>
      </div>

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
                  {logs.map((log, i) => (
                    <tr key={i}>
                      <td className="text-[12px] text-textSecondary whitespace-nowrap">
                        {log.created_at ? format(new Date(log.created_at), 'MM/dd HH:mm:ss') : '-'}
                      </td>
                      <td className="font-semibold text-textPrimary">
                        {log.account_username || `Account #${log.account_id}`}
                      </td>
                      <td>
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-black/5 text-textSecondary dark:bg-white/5">
                          {log.site_name || '-'}
                        </span>
                      </td>
                      <td>
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium ${log.status === 'success' ? 'bg-successSoft text-success' : log.status === 'failed' ? 'bg-dangerSoft text-danger' : 'bg-black/5 text-textSecondary dark:bg-white/5'}`}>
                          {log.status === 'success' ? '成功' : log.status === 'failed' ? '失败' : '跳过'}
                        </span>
                      </td>
                      <td>
                        {log.failureReason ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium bg-infoSoft text-info" title={log.failureReason.actionHint}>
                            {log.failureReason.title}
                          </span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium bg-black/5 text-textSecondary dark:bg-white/5">-</span>
                        )}
                      </td>
                      <td className="max-w-[360px]">
                        <span className="block overflow-hidden text-ellipsis whitespace-nowrap" title={log.message}>
                          {log.status === 'failed' ? (
                            <span className="inline-flex items-center px-1.5 py-0.5 rounded-sm text-[11px] font-medium bg-danger text-white mr-1.5">Error</span>
                          ) : log.status === 'success' && log.reward ? (
                            <span className="inline-flex items-center px-1.5 py-0.5 rounded-sm text-[11px] font-medium bg-success text-white mr-1.5">奖励: {log.reward}</span>
                          ) : null}
                          <span className="text-[12px] font-mono">{log.message}</span>
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {logs.length === 0 && (
              <div className="flex flex-col items-center justify-center p-12 text-center">
                <svg className="w-12 h-12 text-textMuted mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="text-[15px] font-medium text-textPrimary">暂无匹配的签到日志</div>
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
                      <td className="text-[12px] text-textSecondary whitespace-nowrap">
                        {ev.created_at ? format(new Date(ev.created_at), 'MM/dd HH:mm:ss') : '-'}
                      </td>
                      <td>
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium bg-infoSoft text-info">{ev.type}</span>
                      </td>
                      <td>
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[12px] font-medium ${ev.level === 'error' ? 'bg-dangerSoft text-danger' : ev.level === 'warning' ? 'bg-warningSoft text-warning' : 'bg-successSoft text-success'}`}>
                          {ev.level}
                        </span>
                      </td>
                      <td className="font-medium text-textPrimary">
                        {ev.title}
                      </td>
                      <td className="max-w-[400px]">
                        <span className="block overflow-hidden text-ellipsis whitespace-nowrap text-textSecondary" title={ev.message || ''}>
                          {ev.message || '-'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {events.length === 0 && (
              <div className="flex flex-col items-center justify-center p-12 text-center">
                <svg className="w-12 h-12 text-textMuted mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div className="text-[15px] font-medium text-textPrimary">暂无系统事件</div>
              </div>
            )}
          </>
        )}
      </div>
      {!loading && activeTotal > 0 && (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-4 text-[13px] text-textSecondary">
          <div>
            第 <span className="font-mono text-textPrimary">{activePage}</span> / <span className="font-mono text-textPrimary">{totalPages}</span> 页，
            共 <span className="font-mono text-textPrimary">{activeTotal}</span> 条
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className={btnSecondaryClass}
              disabled={activePage <= 1}
              onClick={() => setActivePage(activePage - 1)}
            >
              上一页
            </button>
            <button
              type="button"
              className={btnSecondaryClass}
              disabled={activePage >= totalPages}
              onClick={() => setActivePage(activePage + 1)}
            >
              下一页
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
