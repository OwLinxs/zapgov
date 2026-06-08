-- Mídia: imagem/áudio/documento/vídeo recebidos e enviados.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_type TEXT NOT NULL DEFAULT '';   -- image|audio|video|document|sticker
ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_id   TEXT NOT NULL DEFAULT '';   -- id do arquivo na Meta
ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_mime TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_name TEXT NOT NULL DEFAULT '';
