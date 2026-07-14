# Cover Art Guide — MyNovel.pro

## Quy tắc tuyệt đối

### ❌ KHÔNG BAO GIỜ có chữ trên ảnh bìa
- MyNovel.pro **từ chối** ảnh bìa có text/chữ/watermark
- Mọi prompt PHẢI có: `No text, no title, no letters, no words, no watermark, no logo, no typography`
- Kể cả biển hiệu trong scene (ví dụ "Casa Ramírez" trên quán) → xóa hoặc blur
- Nếu cần sign/biển → dùng `illegible old sign` hoặc `weathered sign with no readable text`

### ❌ KHÔNG vũ khí / đạo cụ nguy hiểm
- Prompt 1 của El sabor de volver có "carries a chef's knife" → render ra giống sát thủ
- Nếu cần dao bếp → chỉ khi nằm trên bàn, không cầm tay

---

## Visual Style — "Golden Triana"

Style đã test thành công với El sabor de volver. Dùng làm baseline cho các cuốn sau.

### Đặc trưng style

| Element | Mô tả |
|---------|-------|
| **Lighting** | Golden hour, cinematic, warm amber dominant |
| **Mood** | Nostalgic, bittersweet, intimate |
| **Color palette** | Deep amber, burnt orange, terracotta, twilight blue shadows |
| **Texture** | Wet cobblestones reflecting light, weathered stucco walls, aged wood |
| **Composition** | Subject from behind (no face), walking toward a destination |
| **Particles** | Floating petals/leaves catching light — adds magic realism |
| **Background** | Blurred architectural depth (church spire, narrow street, lanterns) |
| **Foreground subject** | Woman in flowing skirt, dark top, messy updo — identifiable but anonymous |
| **Secondary element** | Male silhouette visible through doorway/window — implies relationship |
| **Rendering** | Photorealistic with painterly soft edges |
| **Aspect ratio** | 2:3 portrait (vertical) for book cover |

### Negative prompts (luôn thêm)
```
No text, no title, no letters, no words, no watermark, no logo, no typography,
no weapons, no knives in hand, no blood, no nudity, no cartoon style,
no anime, no illustration style
```

---

## Prompt Template — Book Cover

```
Vertical book cover, cinematic golden hour lighting. [SCENE DESCRIPTION].
[FLOATING PARTICLES — petals/leaves/rain catching warm light].
In the foreground, [SUBJECT from behind, no face visible, identifiable by clothing/hair].
[DESTINATION — building/door/path with warm light spilling out].
[SECONDARY FIGURE — silhouette barely visible, implying relationship].
Mood: [2-3 mood words]. Color palette: deep amber, burnt orange, [accent color], twilight blue shadows.
Photorealistic with painterly soft edges. Aspect ratio 2:3 portrait.
No text, no title, no letters, no words, no watermark, no logo.
```

### Ví dụ đã dùng — El sabor de volver

```
Vertical book cover, cinematic lighting. A narrow cobblestone street in
Triana, Seville at golden hour. Orange blossom petals drift through warm
amber light. In the foreground, a woman seen from behind walks toward a
small traditional Spanish bar with a faded green-and-white striped awning.
Her hands hang at her sides, one hand touching the fabric of her skirt.
Through the bar's open door, warm kitchen light spills onto the wet stones.
A man's silhouette is barely visible inside, standing behind a counter.
Mood: nostalgic, bittersweet, warm. Color palette: deep amber, burnt orange,
twilight blue shadows. Photorealistic with painterly edges. Aspect ratio
2:3 portrait. No text, no title, no letters, no words, no watermark, no logo.
```

---

## Prompt Template — Author Avatar

```
Square profile picture, 1:1 ratio. [SYMBOLIC OBJECTS on rustic surface].
[WARM LIGHTING — candle/golden hour]. Background completely dark.
Mood: intimate, secretive, literary. Color palette: warm amber, [accent],
deep black. Painterly style with soft focus edges. No face visible.
No text, no letters, no words.
```

---

## Platform Specs — MyNovel.pro

| Spec | Value |
|------|-------|
| Cover format | Image upload (JPG/PNG) |
| Text on cover | ❌ FORBIDDEN — auto-rejected |
| 18+ content flag | Checkbox (separate from cover) |
| Synopsis limit | 800 ký tự mỗi ngôn ngữ |
| Foreword | 30,000 ký tự max |
