import ws from 'k6/ws';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 0 },
  ],
};

export default function () {
  const url = 'ws://localhost:8080/ws';
  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      socket.send(JSON.stringify({
        type: 'create_room',
        bossId: 'raid_gladiator',
        maxPlayers: 4,
        difficulty: 'normal',
        ownerName: 'Tester',
        ownerId: 'test_' + __VU + '_' + __ITER,
      }));
    });

    socket.on('message', (msg) => {
      const data = JSON.parse(msg);
      if (data.type === 'room_created') {
        socket.send(JSON.stringify({
          type: 'disband_room',
          roomId: data.room.id,
        }));
      }
    });

    socket.on('close', () => {});
    socket.setTimeout(() => socket.close(), 5000);
  });

  check(res, { 'WebSocket connected': (r) => r && r.status === 101 });
}