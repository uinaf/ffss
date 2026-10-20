export interface Mailer {
  send(to: string, body: string): Promise<void>;
}

export async function notify(mailer: Mailer, recipients: string[], body: string): Promise<number> {
  let sent = 0;
  recipients.forEach(async (to) => {
    await mailer.send(to, body);
    sent += 1;
  });
  return sent;
}
