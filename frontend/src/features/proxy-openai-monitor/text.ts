export const copy = {
  zh: {
    monitor: '监测', title: 'OpenAI 延迟监测',
    description: '后台持续采样，默认关闭；仅请求 OpenAI models 接口，不发送账号密钥。延迟从发起请求到响应头返回，复用连接；200/401 为成功，不代表模型速度。失败不记为 0ms；慢请求或积压会跳轮，不补采。原始数据保留 7 天。',
    enabled: '开启监测', interval: '间隔', save: '保存配置', saving: '保存中…', saved: '配置已保存，后台调度已更新。',
    error: '请求失败，请刷新或重试保存。', status: '状态', range: '查询区间', hour: '近 1 小时', day: '近 24 小时', week: '近 7 天',
    refresh: '刷新', autoRefresh: '仅打开时每 15 秒刷新；翻页固定区间，首页获取最新样本', loading: '加载中…',
    noCurrent: '当前配置暂无样本', stale: '最近样本已过期', fresh: '最近样本有效', last: '最近采样',
    samples: '已完成样本', rate: '成功率', average: '成功平均延迟', failures: '失败次数', coverage: '实际样本覆盖',
    empty: '该区间暂无样本；暂停、停机及跳轮均不计成败。', allFailed: '该区间样本全部失败，延迟统计为空。',
    granularity: '趋势粒度', minute: '1 分钟', hourly: '1 小时', time: '采样时间', result: '结果', latency: '延迟', success: '成功', previous: '上一页', next: '下一页',
    staleRule: '超过 3 个采样周期 + 16 秒（当前 {seconds} 秒）视为过期；趋势空白表示缺测。暂停后历史仍按 7 天清理。',
    states: { paused: '已暂停', waiting: '等待采样', running: '采样中', skipped: '已跳过过期轮次', lease_recovered: '租约恢复，等待下轮', unavailable: '代理停用或过期' }
  },
  en: {
    monitor: 'Monitor', title: 'OpenAI latency monitor',
    description: 'Opt-in background sampling of the OpenAI models endpoint without account keys. Measures request to response headers with connection reuse. HTTP 200/401 succeeds; this is not model speed. Failed latency is null. Slow or overdue rounds are skipped, never replayed. Raw samples are kept for 7 days.',
    enabled: 'Enable', interval: 'Interval', save: 'Save configuration', saving: 'Saving…', saved: 'Configuration saved; background schedule updated.',
    error: 'Request failed. Refresh or retry saving.', status: 'Status', range: 'Time range', hour: 'Last hour', day: 'Last 24 hours', week: 'Last 7 days',
    refresh: 'Refresh', autoRefresh: 'Refreshes every 15s while open; later pages keep a fixed range, page 1 shows latest', loading: 'Loading…',
    noCurrent: 'No samples for current connection', stale: 'Latest sample is stale', fresh: 'Latest sample is fresh', last: 'Latest sample',
    samples: 'Completed samples', rate: 'Success rate', average: 'Success mean latency', failures: 'Failures', coverage: 'Actual sample coverage',
    empty: 'No samples in this range. Pauses, downtime and skipped rounds do not count as failures.', allFailed: 'All samples failed; latency metrics are empty.',
    granularity: 'Trend buckets', minute: '1 minute', hourly: '1 hour', time: 'Sample time', result: 'Result', latency: 'Latency', success: 'Success', previous: 'Previous', next: 'Next',
    staleRule: 'Stale after 3 periods + 16s ({seconds}s now). Gaps mean missing samples. Paused history still expires after 7 days.',
    states: { paused: 'Paused', waiting: 'Waiting', running: 'Sampling', skipped: 'Overdue round skipped', lease_recovered: 'Lease recovered; waiting', unavailable: 'Proxy inactive or expired' }
  }
}
