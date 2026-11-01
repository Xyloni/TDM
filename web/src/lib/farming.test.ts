import { describe, expect, it } from 'vitest';
import type { FarmingAccount } from './api/types';
import { sortFarming } from './farming';

const account = (id: string, rate: number | null, reason = 'healthy', eta: number | null = 10): FarmingAccount => ({
	id, name: id, platform: 'twitch', group: 1, channel: 'live', reward: 'Reward', reason,
	stats: { minutes_15: { gain: 15, observation_minutes: 15, rate, ready: rate !== null }, minutes_30: { gain: 0, observation_minutes: 0, rate: null, ready: false }, rotations_15: 0 },
	remaining: 10, eta_minutes: eta, complete: reason === 'complete'
});

describe('farming sorting', () => {
	it('puts slow and broken accounts ahead of healthy, warming and finished accounts', () => {
		const rows = [account('finished', null, 'complete', 0), account('healthy', 1), account('warm', null, 'warming'), account('slow', .8, 'slow'), account('login', null, 'needs_login', null)];
		expect(sortFarming(rows, 'slowest').map(a => a.id)).toEqual(['slow', 'login', 'healthy', 'warm', 'finished']);
		expect(rows[0].id).toBe('finished');
	});
	it('puts unknown completion times before the longest known ones', () => {
		expect(sortFarming([account('short', 1, 'healthy', 5), account('unknown', null, 'warming', null), account('long', .8, 'slow', 30)], 'eta').map(a => a.id)).toEqual(['unknown', 'long', 'short']);
	});
});
