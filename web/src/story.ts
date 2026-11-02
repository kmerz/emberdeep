// The backstory shown in the lobby. Edit freely: each entry is one page, wrapped to the box automatically.
// The names here match the game (floors, bosses, the gnome village), so keep them in step if those change.

export const STORY_TITLE = 'THE LEGEND OF EMBERDEEP';

export const STORY: string[] = [
  'Long ago the town of Ashfall was built above a sleeping fire. Its people raised a cathedral over the vent and ' +
    'lit ten thousand candles in it: as long as the candles burned, the deep stayed asleep. The man who made ' +
    'and tended them was a chandler the townsfolk fondly called the Tallow King.',

  'But the Tallow King grew greedy. He began to melt the town\'s dead into his wax, and his candles burned black. ' +
    'With every black candle the fire below stirred, until one night the whole Cathedral of Ash sank into the ' +
    'earth with its congregation still inside. The dead walk its aisles now, and the King still keeps his candles ' +
    'lit, five floors down.',

  'Beneath the cathedral the old crypts flooded. Ants the size of dogs built their hive among the bones. Further ' +
    'down still, the gnomes of Spoonhollow dug themselves a village (with spoons, they insist) and trade with ' +
    'anyone brave enough to reach them. Ask for Grimble. Bring garnets.',

  'Below the gnomes, nothing is mapped. They speak of caverns where the fungus glows, of a hollow so black that ' +
    'light itself gets lost, and of a gate where things that smile at you want something in return. Never trust ' +
    'a chest down there. Keep your lantern full.',

  'Ashfall once prayed to the Four: Korrath the Anvil, Vess Who Walks Unseen, Ilun of the Burning Eye and Sabe ' +
    'the Still Water. Their altars are still down there, cold and waiting. The gods are tired, not gone: prove ' +
    'yourself in their trials and they will make you stronger. Somewhere, a chandler left a confession.',

  'At the very bottom the fire has a name: the Ember Dragon. It woke when the first black candle was lit, and ' +
    'every year since, Ashfall has grown warmer. The wells steam. The church bells have cracked. Last winter it ' +
    'did not snow at all.',

  'The mayor posted a reward that no one has ever claimed: go down all ten floors, put out the fire at the heart ' +
    'of Emberdeep, and come back up. Many went. A few returned, and none of them would speak of the seventh floor.',

  'You are the next ones. Gather your party, choose your path, and take the stairs. The candles are still ' +
    'burning, and somewhere far below, something is breathing.',
];

/** Breaks text into lines of at most width characters, at spaces. */
export function wrap(text: string, width: number): string[] {
  const out: string[] = [];
  let line = '';
  for (const word of text.split(' ')) {
    if (line && line.length + 1 + word.length > width) { out.push(line); line = word; }
    else line = line ? line + ' ' + word : word;
  }
  if (line) out.push(line);
  return out;
}
