// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The look of the window: the project's dark themes, as Qt style sheets and drawn
// icons. The colours come from the same theme files WittyMediaServer uses.
#pragma once

#include <QColor>
#include <QHash>
#include <QJsonArray>
#include <QJsonObject>
#include <QVector>
#include <QIcon>
#include <QString>
#include <QStringList>

class QApplication;
class QPainter;
class QRectF;

namespace wmp {

// How a theme is finished: gloss on buttons, glass on panels, a bevel on bars. Each is none, subtle or
// full. The server draws these per experience; the player has no experiences, so a theme carries its own
// look: its `look` block when it has one, else what its `finish` block and id say (Themes::lookOf).
struct Finish {
    QString glass = QStringLiteral("none"), gloss = QStringLiteral("none"), bevel = QStringLiteral("none");
    QColor glassHi, glassLo, glassEdge;
    QVector<QColor> glossStops;
    bool any() const { return glass != QLatin1String("none") || gloss != QLatin1String("none") || bevel != QLatin1String("none"); }
};

struct Theme {
    QString id, name, description, scheme, font;
    bool imported = false;
    Finish finish;
    QJsonArray scene; // the theme's backdrop: gradient, radial, shape layers
    int radiusButton = 10, radiusCard = 10, radiusDialog = 20;
    QColor bg, bg2, bg3, bg4;
    QColor fg, fg2, fg3;
    QColor accent, accentHover, accentFg, accentText;
    QColor danger, ok, warn, controlLine, focus;
    int radius = 10;
};

// How two colours read against each other: 1 to 21.
double contrastRatio(const QColor &a, const QColor &b);

// The result of importing a theme ZIP.
struct ThemeImport {
    bool ok = false;
    QString id, name, error;
    QStringList notes; // things that were left out: a font that wouldn't load
};

class Themes {
public:
    // The fifteen built-in themes, in the order they are offered: the server's.
    static QStringList ids();
    // Themes the viewer imported, and both together (imported ones first).
    static QStringList importedIds();
    static QStringList allIds();
    static Theme load(const QString &id);
    // The theme an id from an earlier version of the player is now: the closest of the fifteen.
    static QString migrate(const QString &id);
    // Reads a ZIP of theme.json and theme.css (and fonts and pictures) by the server's rules
    // (WittyMediaServer docs/THEMES.md): variables and @font-face only, up to 25 MB, 200 files.
    static ThemeImport importZip(const QString &zipPath);
    static bool removeImported(const QString &id);
    static QString importDir();
    // Colours and sizes as CSS writes them: #rgb, #rrggbb, #rrggbbaa, rgb(), rgba().
    static QColor cssColor(const QString &text, const QColor &fallback = QColor());
    // Whether the finishes are drawn: off, a theme is flat.
    static void setFlat(bool flat);
    static bool flat();
    // The theme's backdrop, drawn into a rectangle.
    static void paintScene(QPainter *p, const Theme &t, const QRectF &r);
    // Extra rules a finished look adds to the usual style sheet (gloss, glass, bevel).
    static QString finishSheet(const Theme &t);
    static QString styleSheet(const Theme &t);
    static void apply(QApplication *app, const Theme &t);
    // What the window's frame and title bar should be for this theme: Windows draws them from it.
    static Qt::ColorScheme colorScheme(const Theme &t);
    static const Theme &current();
    // An icon from the small drawn set: play pause prev next volume mute fullscreen
    // exitfullscreen audio subtitles menu title stop open disc.
    static QIcon icon(const QString &name);
};

} // namespace wmp
