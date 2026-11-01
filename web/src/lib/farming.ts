import type { FarmingAccount } from '$lib/api/types';

export const farmReasonKeys: Record<string, string> = {
	paused: 'farm.reason.paused', waiting_group: 'farm.reason.waiting_group', waiting_scan: 'farm.reason.waiting_scan',
	needs_login: 'farm.reason.needs_login', no_games: 'farm.reason.no_games', claiming: 'farm.reason.claiming',
	complete: 'farm.reason.complete', blocked: 'farm.reason.blocked', waiting_stream: 'farm.reason.waiting_stream',
	force_watching: 'farm.reason.force_watching', starting: 'farm.reason.starting', stale: 'farm.reason.stale',
	warming: 'farm.reason.warming', slow: 'farm.reason.slow', healthy: 'farm.reason.healthy'
};
export function problemAccount(a: FarmingAccount): boolean {
	return ['slow', 'stale', 'needs_login', 'blocked'].includes(a.reason);
}
export function sortFarming(rows: FarmingAccount[], by: string): FarmingAccount[] {
	return [...rows].sort((a,b) => {
		if (by === 'name') return a.name.localeCompare(b.name);
		if (by === 'eta') return (b.eta_minutes ?? Infinity) - (a.eta_minutes ?? Infinity) || a.name.localeCompare(b.name);
		const priority = (x: FarmingAccount) => problemAccount(x) ? 0 : x.complete || ['paused', 'waiting_group'].includes(x.reason) ? 2 : 1;
		return priority(a)-priority(b) || (a.stats.minutes_15.rate ?? Infinity)-(b.stats.minutes_15.rate ?? Infinity) || a.name.localeCompare(b.name);
	});
}
