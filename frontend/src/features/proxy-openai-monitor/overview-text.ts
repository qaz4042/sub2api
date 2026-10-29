export const overviewCopy = {
  zh: {
    title: '代理延迟总览', back: '代理管理', entry: '延迟总览',
    description: '后台 OpenAI 连通性监测；仅统计当前连接配置。平均延迟、P95 仅含成功样本，失败不折算为 0ms。暂停后保留历史；停机、跳轮和未采样不计失败。',
    search: '搜索代理名称', searchButton: '搜索', range: '统计区间', filter: '监测筛选', sort: '排序',
    all: '全部代理', enabled: '监测已开启', paused: '监测已关闭',
    autoRefresh: '可见时每 15 秒刷新', updated: '更新于', proxies: '个代理',
    error: '总览更新失败，请重试；已显示的数据为上次成功结果。', empty: '没有符合条件的代理',
    name: '代理名称', state: '监测状态', latest: '最新延迟', samples: '样本 / 失败', details: '详情 / 配置', failed: '失败',
    statuses: { unavailable: '停用 / 过期', paused: '已暂停', empty: '当前配置暂无样本', stale: '样本过期', success: '连通正常', failed: '连通失败' },
    sorts: { latency: '最新延迟：高到低', failures: '失败次数：多到少', p95: 'P95：高到低', success_rate: '成功率：低到高', name: '代理名称' }
  },
  en: {
    title: 'Proxy latency overview', back: 'Manage proxies', entry: 'Latency overview',
    description: 'Background OpenAI connectivity monitoring for the current connection only. Mean and P95 include successful samples; failures are never zero latency. Paused history is retained. Downtime, skipped rounds and missing samples are not failures.',
    search: 'Search proxy names', searchButton: 'Search', range: 'Time range', filter: 'Monitor filter', sort: 'Sort',
    all: 'All proxies', enabled: 'Monitoring enabled', paused: 'Monitoring off',
    autoRefresh: 'Refreshes every 15s while visible', updated: 'Updated', proxies: 'proxies',
    error: 'Overview refresh failed. Retry; any displayed data is from the last successful request.', empty: 'No matching proxies',
    name: 'Proxy name', state: 'Monitor status', latest: 'Latest latency', samples: 'Samples / failures', details: 'Details / settings', failed: 'Failed',
    statuses: { unavailable: 'Inactive / expired', paused: 'Paused', empty: 'No current samples', stale: 'Stale', success: 'Reachable', failed: 'Failed' },
    sorts: { latency: 'Latest latency: high to low', failures: 'Failures: high to low', p95: 'P95: high to low', success_rate: 'Success rate: low to high', name: 'Proxy name' }
  }
}
