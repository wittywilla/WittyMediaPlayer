// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
#include "theme.h"

#include <QApplication>
#include <QDir>
#include <QFileInfo>
#include <QFontDatabase>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStandardPaths>
#include <private/qzipreader_p.h>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPainter>
#include <QPainterPath>
#include <QPalette>
#include <QPixmap>
#include <QStyleHints>
#include <cmath>

namespace wmp {

namespace {
Theme g_current;

// Theme files write translucent colours as #rrggbbaa; only opaque ones are used here.
QColor hexColor(const QJsonObject &c, const char *key, const QColor &fallback)
{
    const QString v = c.value(QLatin1String(key)).toString();
    if (v.size() != 7 && v.size() != 9)
        return fallback;
    QColor out(v.left(7));
    return out.isValid() ? out : fallback;
}

double channel(double v)
{
    v /= 255.0;
    return v <= 0.03928 ? v / 12.92 : std::pow((v + 0.055) / 1.055, 2.4);
}

double luminance(const QColor &c)
{
    return 0.2126 * channel(c.red()) + 0.7152 * channel(c.green()) + 0.0722 * channel(c.blue());
}
} // namespace

double contrastRatio(const QColor &a, const QColor &b)
{
    double x = luminance(a), y = luminance(b);
    if (x < y)
        std::swap(x, y);
    return (x + 0.05) / (y + 0.05);
}

QStringList Themes::ids()
{
    return {QStringLiteral("default"), QStringLiteral("midnight"), QStringLiteral("cinema"), QStringLiteral("evergreen"), QStringLiteral("oled"),
            QStringLiteral("light"), QStringLiteral("paper"), QStringLiteral("contrast"), QStringLiteral("aeroday"), QStringLiteral("aeroaurora"),
            QStringLiteral("aeromeadow"), QStringLiteral("aerosunset"), QStringLiteral("cable"), QStringLiteral("satellite"), QStringLiteral("teletext")};
}

QString Themes::migrate(const QString &id)
{
    static const QHash<QString, QString> old = {
        {QStringLiteral("dark"), QStringLiteral("default")},      {QStringLiteral("dark-blue"), QStringLiteral("midnight")},
        {QStringLiteral("dark-teal"), QStringLiteral("evergreen")}, {QStringLiteral("dark-green"), QStringLiteral("evergreen")},
        {QStringLiteral("dark-gold"), QStringLiteral("cinema")},    {QStringLiteral("dark-orange"), QStringLiteral("cinema")},
        {QStringLiteral("dark-red"), QStringLiteral("cinema")},     {QStringLiteral("dark-pink"), QStringLiteral("default")},
        {QStringLiteral("dark-purple"), QStringLiteral("default")}};
    return old.value(id, id);
}

QColor Themes::cssColor(const QString &text, const QColor &fallback)
{
    const QString v = text.trimmed().toLower();
    if (v.startsWith(QLatin1Char('#'))) {
        QString hex = v.mid(1);
        if (hex.size() == 3 || hex.size() == 4) {
            QString full;
            for (const QChar ch : hex)
                full += QString(2, ch);
            hex = full;
        }
        bool ok = false;
        if (hex.size() == 6) {
            const uint rgb = hex.toUInt(&ok, 16);
            return ok ? QColor::fromRgb(rgb) : fallback;
        }
        if (hex.size() == 8) {
            const uint rgba = hex.toUInt(&ok, 16);
            return ok ? QColor(int((rgba >> 24) & 255), int((rgba >> 16) & 255), int((rgba >> 8) & 255), int(rgba & 255)) : fallback;
        }
        return fallback;
    }
    if (v.startsWith(QLatin1String("rgb"))) {
        const int open = v.indexOf(QLatin1Char('(')), close = v.lastIndexOf(QLatin1Char(')'));
        if (open < 0 || close < open)
            return fallback;
        const QStringList parts = v.mid(open + 1, close - open - 1).split(QRegularExpression(QStringLiteral(R"([\s,/]+)")), Qt::SkipEmptyParts);
        if (parts.size() < 3)
            return fallback;
        const auto num = [](const QString &s) { return s.endsWith(QLatin1Char('%')) ? s.chopped(1).toDouble() * 2.55 : s.toDouble(); };
        double alpha = 1;
        if (parts.size() > 3)
            alpha = parts[3].endsWith(QLatin1Char('%')) ? parts[3].chopped(1).toDouble() / 100 : parts[3].toDouble();
        return QColor(qBound(0, int(num(parts[0])), 255), qBound(0, int(num(parts[1])), 255), qBound(0, int(num(parts[2])), 255), qBound(0, int(alpha * 255 + 0.5), 255));
    }
    return fallback;
}

namespace {
bool g_flat = false;

// What the finishes of a theme are. A `look` block in theme.json says so outright (none, subtle, full
// for glass, gloss and bevel); otherwise a theme with a `finish` block (the Aero themes) is glass and
// gloss with a light bevel, and the two classic ones are bevelled bars.
Finish finishOf(const QString &id, const QJsonObject &o)
{
    Finish f;
    const QJsonObject fin = o.value(QStringLiteral("finish")).toObject();
    const QJsonObject glass = fin.value(QStringLiteral("glass")).toObject();
    f.glassHi = Themes::cssColor(glass.value(QStringLiteral("hi")).toString(), QColor(255, 255, 255, 200));
    f.glassLo = Themes::cssColor(glass.value(QStringLiteral("lo")).toString(), QColor(255, 255, 255, 110));
    f.glassEdge = Themes::cssColor(glass.value(QStringLiteral("edge")).toString(), QColor(255, 255, 255));
    for (const auto &v : fin.value(QStringLiteral("gloss")).toArray()) {
        const QColor c = Themes::cssColor(v.toString());
        if (c.isValid())
            f.glossStops.push_back(c);
    }
    const bool hasFinish = !fin.isEmpty();
    if (hasFinish) {
        f.glass = QStringLiteral("full");
        f.gloss = f.glossStops.size() >= 2 ? QStringLiteral("full") : QStringLiteral("none");
        f.bevel = QStringLiteral("subtle");
    }
    if (id == QLatin1String("satellite"))
        f.bevel = QStringLiteral("full");
    else if (id == QLatin1String("cable"))
        f.bevel = QStringLiteral("subtle");
    const QJsonObject look = o.value(QStringLiteral("look")).toObject();
    for (auto *slot : {&f.glass, &f.gloss, &f.bevel}) {
        const QString key = slot == &f.glass ? QStringLiteral("glass") : slot == &f.gloss ? QStringLiteral("gloss") : QStringLiteral("bevel");
        const QString v = look.value(key).toString();
        if (v == QLatin1String("none") || v == QLatin1String("subtle") || v == QLatin1String("full"))
            *slot = v;
    }
    return f;
}

Theme fromJson(const QString &id, const QJsonObject &o, bool imported)
{
    Theme t;
    t.id = id;
    t.imported = imported;
    const QJsonObject c = o.value(QStringLiteral("colors")).toObject();
    t.name = o.value(QStringLiteral("name")).toString(id);
    t.description = o.value(QStringLiteral("description")).toString();
    t.scheme = o.value(QStringLiteral("scheme")).toString(QStringLiteral("dark"));
    t.bg = hexColor(c, "bg", QColor(0x101114));
    t.bg2 = hexColor(c, "bg2", QColor(0x17191e));
    t.bg3 = hexColor(c, "bg3", QColor(0x20232a));
    t.bg4 = hexColor(c, "bg4", QColor(0x2b2f38));
    t.fg = hexColor(c, "fg", QColor(0xececf1));
    t.fg2 = hexColor(c, "fg2", QColor(0xa8adb9));
    t.fg3 = hexColor(c, "fg3", QColor(0x858a99));
    t.accent = hexColor(c, "accent", QColor(0xe4e6eb));
    t.accentHover = hexColor(c, "accentHover", t.accent);
    t.accentFg = hexColor(c, "accentFg", QColor(0x111214));
    t.accentText = hexColor(c, "accentText", t.accent);
    t.danger = hexColor(c, "danger", QColor(0xff5c6c));
    t.ok = hexColor(c, "ok", QColor(0x3ad29f));
    t.warn = hexColor(c, "warn", QColor(0xe0a030));
    t.controlLine = hexColor(c, "controlLine", QColor(0x6f7480));
    t.focus = hexColor(c, "focus", t.accent);
    t.radius = o.value(QStringLiteral("radius")).toInt(10);
    const QJsonObject shape = o.value(QStringLiteral("shape")).toObject();
    t.radiusButton = shape.value(QStringLiteral("button")).toInt(t.radius);
    t.radiusCard = shape.value(QStringLiteral("card")).toInt(t.radius);
    t.radiusDialog = shape.value(QStringLiteral("dialog")).toInt(t.radius * 2);
    const QJsonArray body = o.value(QStringLiteral("type")).toObject().value(QStringLiteral("body")).toArray();
    if (!body.isEmpty())
        t.font = body.first().toString();
    t.scene = o.value(QStringLiteral("scene")).toObject().value(QStringLiteral("layers")).toArray();
    t.finish = finishOf(id, o);
    return t;
}
} // namespace

void Themes::setFlat(bool flat) { g_flat = flat; }
bool Themes::flat() { return g_flat; }

Theme Themes::load(const QString &requested)
{
    const QString id = migrate(requested);
    QFile f(QStringLiteral(":/themes/%1.json").arg(id));
    bool imported = false;
    if (!f.exists() || !ids().contains(id)) {
        f.setFileName(QDir(importDir()).filePath(id + QStringLiteral("/theme.json")));
        imported = true;
    }
    QJsonObject o;
    if (f.open(QIODevice::ReadOnly))
        o = QJsonDocument::fromJson(f.readAll()).object();
    if (o.isEmpty() && imported) // an imported theme that has gone: the house look
        return load(QStringLiteral("default"));
    return fromJson(id, o, imported);
}

QString Themes::styleSheet(const Theme &t)
{
    const auto h = [](const QColor &c) { return c.name(QColor::HexRgb); };
    QString s = QStringLiteral(R"(
* { font-size: 14px; }
QMainWindow, QDialog, QWidget#central, QWidget#startPage { background: @bg; color: @fg; }
QWidget#controlBar { background: @bg2; }
QLabel { color: @fg; background: transparent; }
QLabel[role="muted"] { color: @fg2; }
QLabel[role="hint"] { color: @fg3; font-size: 13px; }
QLabel[role="brand"] { font-size: 38px; font-weight: 650; }
QLabel[role="heading"] { font-size: 17px; font-weight: 600; }
QLabel[role="time"] { color: @fg2; font-size: 13px; }
QLabel[role="message"] { font-size: 15px; }
QLabel[role="notice"] { background: @bg2; color: @fg; border: 1px solid @bg4; border-radius: @radiuspx; padding: 8px 14px; font-size: 15px; }
QWidget#messageBox { background: @bg2; border: 1px solid @bg4; border-radius: @radiuspx; }
QWidget#driveRow { background: @bg3; border-radius: @radiuspx; }
QMenuBar { background: @bg2; color: @fg2; padding: 2px 6px; }
QMenuBar::item { background: transparent; padding: 6px 10px; border-radius: 6px; }
QMenuBar::item:selected { background: @bg3; color: @fg; }
QMenu { background: @bg2; color: @fg; border: 1px solid @bg4; padding: 6px; }
QMenu::item { padding: 7px 28px 7px 24px; border-radius: 6px; }
QMenu::item:selected { background: @bg3; }
QMenu::item:disabled { color: @fg3; }
QMenu::separator { height: 1px; background: @bg4; margin: 6px 8px; }
QMenu::indicator { width: 14px; height: 14px; left: 6px; }
QRadioButton, QCheckBox { color: @fg; spacing: 8px; }
QCheckBox:focus { color: @accentText; }
QRadioButton::indicator, QCheckBox::indicator { width: 14px; height: 14px; background: @bg3; border: 2px solid @line; }
QRadioButton::indicator { border-radius: 9px; }
QCheckBox::indicator { border-radius: 4px; }
QRadioButton::indicator:checked, QCheckBox::indicator:checked { background: @accent; border-color: @accent; }
QRadioButton:focus::indicator, QCheckBox:focus::indicator { border-color: @focus; }
QFrame[frameShape="4"] { color: @bg4; }
QRadioButton:focus { color: @accentText; }
QLineEdit { background: @bg3; color: @fg; border: 1px solid @line; border-radius: 6px; padding: 6px 8px; }
QLineEdit:focus { border: 2px solid @focus; padding: 5px 7px; }
QLineEdit:disabled { color: @fg3; border-color: @bg4; }
QPushButton { background: @bg3; color: @fg; border: 1px solid @bg4; border-radius: @radiuspx; padding: 8px 16px; }
QPushButton:hover { background: @bg4; }
QPushButton:pressed { background: @bg4; border-color: @line; }
QPushButton:disabled { color: @fg3; }
QPushButton:focus { border: 2px solid @focus; padding: 7px 15px; }
QPushButton[role="primary"] { background: @accent; color: @accentFg; border-color: @accent; font-weight: 600; }
QPushButton[role="primary"]:hover { background: @accentHover; }
QToolButton { background: transparent; color: @fg; border: none; border-radius: 8px; padding: 6px; }
QToolButton:hover { background: @bg3; }
QToolButton:pressed { background: @bg4; }
QToolButton:disabled { color: @fg3; }
QToolButton:focus { border: 2px solid @focus; padding: 4px; }
QToolButton::menu-indicator { image: none; }
QProgressBar { background: @bg3; border: 1px solid @bg4; border-radius: 6px; text-align: center; color: @fg; min-height: 20px; }
QProgressBar::chunk { background: @accent; border-radius: 5px; }
QToolTip { background: @bg3; color: @fg; border: 1px solid @bg4; padding: 4px 8px; }
QScrollBar:vertical { background: @bg; width: 10px; }
QScrollBar::handle:vertical { background: @bg4; border-radius: 5px; min-height: 24px; }
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }
)");
    const QVector<QPair<QString, QColor>> colors = {
        {QStringLiteral("@bg4"), t.bg4}, {QStringLiteral("@bg3"), t.bg3}, {QStringLiteral("@bg2"), t.bg2}, {QStringLiteral("@bg"), t.bg},
        {QStringLiteral("@fg3"), t.fg3}, {QStringLiteral("@fg2"), t.fg2}, {QStringLiteral("@fg"), t.fg},
        {QStringLiteral("@accentHover"), t.accentHover}, {QStringLiteral("@accentFg"), t.accentFg}, {QStringLiteral("@accentText"), t.accentText}, {QStringLiteral("@accent"), t.accent},
        {QStringLiteral("@line"), t.controlLine}, {QStringLiteral("@focus"), t.focus},
    };
    for (const auto &c : colors)
        s.replace(c.first, h(c.second));
    s.replace(QStringLiteral("@radius"), QString::number(t.radiusButton));
    return s + finishSheet(t);
}

const Theme &Themes::current()
{
    if (g_current.id.isEmpty())
        g_current = load(QStringLiteral("default"));
    return g_current;
}

void Themes::apply(QApplication *app, const Theme &t)
{
    g_current = t;
    QPalette p;
    p.setColor(QPalette::Window, t.bg);
    p.setColor(QPalette::WindowText, t.fg);
    p.setColor(QPalette::Base, t.bg3);
    p.setColor(QPalette::AlternateBase, t.bg2);
    p.setColor(QPalette::Text, t.fg);
    p.setColor(QPalette::Button, t.bg3);
    p.setColor(QPalette::ButtonText, t.fg);
    p.setColor(QPalette::Highlight, t.accent);
    p.setColor(QPalette::HighlightedText, t.accentFg);
    p.setColor(QPalette::ToolTipBase, t.bg3);
    p.setColor(QPalette::ToolTipText, t.fg);
    p.setColor(QPalette::PlaceholderText, t.fg3);
    p.setColor(QPalette::Disabled, QPalette::Text, t.fg3);
    p.setColor(QPalette::Disabled, QPalette::ButtonText, t.fg3);
    p.setColor(QPalette::Disabled, QPalette::WindowText, t.fg3);
    app->setPalette(p);
    // The theme's own typeface first, then the platform's.
    QStringList families;
    if (!t.font.isEmpty())
        families << t.font;
    families << QStringLiteral("Segoe UI Variable Text") << QStringLiteral("Segoe UI") << QStringLiteral("system-ui") << QStringLiteral("sans-serif");
    QFont f = app->font();
    f.setFamilies(families);
    app->setFont(f);
    app->setStyleSheet(styleSheet(t));
    // The window's frame and title bar follow the theme, not what Windows uses for its own apps.
    app->styleHints()->setColorScheme(colorScheme(t));
}

Qt::ColorScheme Themes::colorScheme(const Theme &t)
{
    return luminance(t.bg) < 0.18 ? Qt::ColorScheme::Dark : Qt::ColorScheme::Light;
}

namespace {

// Drawn on a 48-unit grid, at any size, so a screen at 300% gets as sharp a picture as one at 100%.
QPixmap drawIcon(const QString &name, int size, const QColor &c)
{
    QPixmap pm(size, size);
    pm.fill(Qt::transparent);
    QPainter p(&pm);
    p.setRenderHint(QPainter::Antialiasing);
    p.scale(size / 48.0, size / 48.0);
    p.setBrush(c);
    p.setPen(QPen(c, 4, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
    const auto poly = [&](std::initializer_list<QPointF> pts) { p.drawPolygon(QPolygonF(QList<QPointF>(pts))); };
    const auto line = [&](qreal a, qreal b, qreal x, qreal y) { p.drawLine(QPointF(a, b), QPointF(x, y)); };
    if (name == QLatin1String("play")) {
        poly({{15, 9}, {38, 24}, {15, 39}});
    } else if (name == QLatin1String("pause")) {
        p.drawRoundedRect(QRectF(12, 9, 8, 30), 2, 2);
        p.drawRoundedRect(QRectF(28, 9, 8, 30), 2, 2);
    } else if (name == QLatin1String("visualizer")) {
        p.setPen(Qt::NoPen);
        p.drawRoundedRect(QRectF(8, 26, 6, 14), 1, 1);
        p.drawRoundedRect(QRectF(17, 10, 6, 30), 1, 1);
        p.drawRoundedRect(QRectF(26, 20, 6, 20), 1, 1);
        p.drawRoundedRect(QRectF(35, 6, 6, 34), 1, 1);
    } else if (name == QLatin1String("stop")) {
        p.drawRoundedRect(QRectF(12, 12, 24, 24), 3, 3);
    } else if (name == QLatin1String("prev")) {
        poly({{36, 10}, {18, 24}, {36, 38}});
        line(12, 10, 12, 38);
    } else if (name == QLatin1String("next")) {
        poly({{12, 10}, {30, 24}, {12, 38}});
        line(36, 10, 36, 38);
    } else if (name == QLatin1String("volume") || name == QLatin1String("mute")) {
        poly({{7, 19}, {15, 19}, {25, 10}, {25, 38}, {15, 29}, {7, 29}});
        p.setBrush(Qt::NoBrush);
        if (name == QLatin1String("volume")) {
            p.drawArc(QRectF(20, 15, 14, 18), -60 * 16, 120 * 16);
            p.drawArc(QRectF(20, 8, 22, 32), -60 * 16, 120 * 16);
        } else {
            line(32, 18, 42, 30);
            line(42, 18, 32, 30);
        }
    } else if (name == QLatin1String("fullscreen") || name == QLatin1String("exitfullscreen")) {
        p.setBrush(Qt::NoBrush);
        const bool in = name == QLatin1String("exitfullscreen");
        const auto corner = [&](qreal x, qreal y, qreal dx, qreal dy) {
            const qreal s = in ? -1 : 1;
            const qreal ax = in ? x + dx * 10 : x, ay = in ? y + dy * 10 : y;
            line(ax, ay, ax + s * dx * 10, ay);
            line(ax, ay, ax, ay + s * dy * 10);
        };
        corner(9, 9, 1, 1);
        corner(39, 9, -1, 1);
        corner(9, 39, 1, -1);
        corner(39, 39, -1, -1);
    } else if (name == QLatin1String("audio")) {
        p.setBrush(Qt::NoBrush);
        line(18, 34, 18, 10);
        line(18, 10, 38, 7);
        line(38, 7, 38, 31);
        p.setBrush(c);
        p.drawEllipse(QPointF(13, 35), 5, 4.5);
        p.drawEllipse(QPointF(33, 32), 5, 4.5);
    } else if (name == QLatin1String("subtitles")) {
        p.setBrush(Qt::NoBrush);
        p.drawRoundedRect(QRectF(6, 11, 36, 26), 4, 4);
        line(13, 25, 22, 25);
        line(26, 25, 35, 25);
        line(13, 31, 28, 31);
    } else if (name == QLatin1String("menu")) {
        line(9, 13, 39, 13);
        line(9, 24, 39, 24);
        line(9, 35, 39, 35);
    } else if (name == QLatin1String("title")) {
        p.setBrush(Qt::NoBrush);
        p.drawRoundedRect(QRectF(7, 9, 34, 30), 4, 4);
        line(7, 18, 41, 18);
        line(16, 9, 16, 18);
        line(27, 9, 27, 18);
    } else if (name == QLatin1String("open")) {
        p.setBrush(Qt::NoBrush);
        p.drawPolygon(QPolygonF(QList<QPointF>{{6, 12}, {19, 12}, {23, 17}, {42, 17}, {42, 38}, {6, 38}}));
    } else if (name == QLatin1String("disc")) {
        p.setBrush(Qt::NoBrush);
        p.drawEllipse(QPointF(24, 24), 18, 18);
        p.setBrush(c);
        p.drawEllipse(QPointF(24, 24), 4, 4);
    }
    p.end();
    return pm;
}

} // namespace

QIcon Themes::icon(const QString &name)
{
    // Asked for on every volume step, so each is drawn once per theme. Emptied while the
    // application still exists, as pictures mustn't outlive it.
    static QHash<QString, QIcon> made;
    static const bool emptiedAtExit = (qAddPostRoutine([] { made.clear(); }), true);
    Q_UNUSED(emptiedAtExit);
    const QString key = current().id + QLatin1Char('/') + name;
    auto it = made.constFind(key);
    if (it != made.constEnd())
        return *it;
    // A control that can't be used looks it: drawn in the theme's faintest text colour.
    QIcon icon;
    for (const int size : {24, 48, 72, 96}) {
        icon.addPixmap(drawIcon(name, size, current().fg), QIcon::Normal);
        icon.addPixmap(drawIcon(name, size, current().fg3), QIcon::Disabled);
    }
    made.insert(key, icon);
    return icon;
}


// ------------------------------------------------------------------ imported themes

QString Themes::importDir()
{
    return QDir(QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)).filePath(QStringLiteral("themes"));
}

QStringList Themes::importedIds()
{
    QStringList out;
    const QDir d(importDir());
    for (const QString &name : d.entryList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name))
        if (QFile::exists(d.filePath(name + QStringLiteral("/theme.json"))) && !ids().contains(name))
            out << name;
    return out;
}

QStringList Themes::allIds() { return importedIds() + ids(); }

bool Themes::removeImported(const QString &id)
{
    if (!importedIds().contains(id))
        return false;
    return QDir(QDir(importDir()).filePath(id)).removeRecursively();
}

namespace {

constexpr qint64 kMaxZip = 25 << 20, kMaxFile = 10 << 20;
constexpr int kMaxFiles = 200;

QString stripComments(QString css)
{
    css.remove(QRegularExpression(QStringLiteral(R"(/\*.*?\*/)"), QRegularExpression::DotMatchesEverythingOption));
    return css;
}

// What a stylesheet may hold: `:root { --variables; color-scheme }` and `@font-face { ... }`, nothing else. An
// empty string means it is fine; otherwise the first rule that isn't.
QString badRule(const QString &raw, QMap<QString, QString> *vars, QStringList *fontUrls)
{
    const QString css = stripComments(raw);
    int i = 0;
    while (i < css.size()) {
        while (i < css.size() && css[i].isSpace())
            i++;
        if (i >= css.size())
            break;
        const int open = css.indexOf(QLatin1Char('{'), i);
        if (open < 0)
            return css.mid(i, 60).trimmed();
        const QString selector = css.mid(i, open - i).trimmed();
        int close = css.indexOf(QLatin1Char('}'), open);
        if (close < 0)
            return selector;
        const QString body = css.mid(open + 1, close - open - 1);
        if (body.contains(QLatin1Char('{'))) // a nested rule, such as a media query
            return selector;
        const bool root = selector == QLatin1String(":root"), face = selector == QLatin1String("@font-face");
        if (!root && !face)
            return selector;
        for (const QString &decl : body.split(QLatin1Char(';'), Qt::SkipEmptyParts)) {
            const int colon = decl.indexOf(QLatin1Char(':'));
            if (colon < 0)
                return selector + QStringLiteral(" { ") + decl.trimmed() + QStringLiteral(" }");
            const QString prop = decl.left(colon).trimmed().toLower(), value = decl.mid(colon + 1).trimmed();
            if (root && !(prop.startsWith(QLatin1String("--")) || prop == QLatin1String("color-scheme")))
                return QStringLiteral(":root { %1 }").arg(decl.trimmed());
            static const QStringList faceProps = {QStringLiteral("font-family"), QStringLiteral("src"), QStringLiteral("font-weight"), QStringLiteral("font-style"),
                                                  QStringLiteral("font-display"), QStringLiteral("font-stretch"), QStringLiteral("unicode-range")};
            if (face && !faceProps.contains(prop))
                return QStringLiteral("@font-face { %1 }").arg(decl.trimmed());
            // An address may only be a file in the ZIP: nothing that climbs out or leaves the computer.
            static const QRegularExpression url(QStringLiteral(R"(url\(\s*['"]?([^'")]*)['"]?\s*\))"));
            auto it = url.globalMatch(value);
            while (it.hasNext()) {
                const QString target = it.next().captured(1);
                if (target.contains(QLatin1String("..")) || target.contains(QLatin1Char(':')) || target.startsWith(QLatin1Char('/')) || target.startsWith(QLatin1Char('\\')))
                    return QStringLiteral("%1 { %2 }").arg(selector, decl.trimmed());
                if (face && fontUrls)
                    *fontUrls << target;
            }
            if (value.contains(QLatin1String("@import"), Qt::CaseInsensitive) || value.contains(QLatin1String("expression("), Qt::CaseInsensitive))
                return QStringLiteral("%1 { %2 }").arg(selector, decl.trimmed());
            if (root && vars && prop.startsWith(QLatin1String("--")))
                vars->insert(prop, value);
        }
        i = close + 1;
    }
    return {};
}

QString resolveVar(const QMap<QString, QString> &vars, QString value, int depth = 0)
{
    static const QRegularExpression ref(QStringLiteral(R"(^var\(\s*(--[\w-]+)\s*(?:,\s*(.*))?\)$)"));
    const auto m = ref.match(value.trimmed());
    if (!m.hasMatch() || depth > 8)
        return value.trimmed();
    const QString name = m.captured(1);
    if (vars.contains(name))
        return resolveVar(vars, vars.value(name), depth + 1);
    return m.captured(2).isEmpty() ? QString() : resolveVar(vars, m.captured(2), depth + 1);
}

} // namespace

ThemeImport Themes::importZip(const QString &zipPath)
{
    ThemeImport r;
    const QFileInfo zi(zipPath);
    if (!zi.exists() || zi.size() > kMaxZip) {
        r.error = zi.exists() ? QStringLiteral("That ZIP is over 25 MB.") : QStringLiteral("That file isn't there.");
        return r;
    }
    QZipReader zip(zipPath);
    if (!zip.isReadable()) {
        r.error = QStringLiteral("That isn't a ZIP this player can read.");
        return r;
    }
    const auto infos = zip.fileInfoList();
    if (infos.size() > kMaxFiles) {
        r.error = QStringLiteral("That ZIP has more than 200 files.");
        return r;
    }
    QByteArray json, cssBytes;
    for (const auto &fi : infos) {
        if (fi.size > kMaxFile) {
            r.error = QStringLiteral("%1 is over 10 MB.").arg(fi.filePath);
            return r;
        }
        const QString p = fi.filePath;
        if (p.contains(QLatin1String("..")) || p.startsWith(QLatin1Char('/')) || p.contains(QLatin1Char(':')) || p.contains(QLatin1Char('\\'))) {
            r.error = QStringLiteral("%1 isn't a path a theme may have.").arg(p);
            return r;
        }
        if (fi.isSymLink) {
            r.error = QStringLiteral("%1 is a link, which a theme may not hold.").arg(p);
            return r;
        }
        if (p == QLatin1String("theme.json"))
            json = zip.fileData(p);
        else if (p == QLatin1String("theme.css"))
            cssBytes = zip.fileData(p);
    }
    if (json.isEmpty() || cssBytes.isEmpty()) {
        r.error = QStringLiteral("theme.json and theme.css must be at the top of the ZIP, not inside a folder.");
        return r;
    }
    QJsonParseError pe;
    QJsonObject o = QJsonDocument::fromJson(json, &pe).object();
    if (pe.error != QJsonParseError::NoError || o.isEmpty()) {
        r.error = QStringLiteral("theme.json can't be read: %1").arg(pe.errorString());
        return r;
    }
    if (o.value(QStringLiteral("builtin")).toBool()) {
        r.error = QStringLiteral("theme.json says \"builtin\", which is only for the themes that come with the program. Delete that line.");
        return r;
    }
    QMap<QString, QString> vars;
    QStringList fontUrls;
    const QString bad = badRule(QString::fromUtf8(cssBytes), &vars, &fontUrls);
    if (!bad.isEmpty()) {
        r.error = QStringLiteral("theme.css may hold only :root variables and @font-face, and this isn't: %1").arg(bad.left(80));
        return r;
    }
    QString id = o.value(QStringLiteral("id")).toString(QFileInfo(zipPath).completeBaseName()).toLower();
    static const QRegularExpression idOk(QStringLiteral("^[a-z0-9][a-z0-9-]{0,40}$"));
    if (!idOk.match(id).hasMatch()) {
        r.error = QStringLiteral("The id \"%1\" must be lower case letters, digits and hyphens.").arg(id);
        return r;
    }
    if (ids().contains(id)) {
        r.error = QStringLiteral("The id \"%1\" is one of the built-in themes. Choose another in theme.json.").arg(id);
        return r;
    }
    // Colours: theme.json's `colors`, else the stylesheet's variables over the nearest built-in theme.
    QJsonObject base = QJsonDocument::fromJson([&] {
                           QFile f(QStringLiteral(":/themes/%1.json").arg(o.value(QStringLiteral("scheme")).toString() == QLatin1String("light") ? QStringLiteral("light") : QStringLiteral("default")));
                           f.open(QIODevice::ReadOnly);
                           return f.readAll();
                       }()).object();
    QJsonObject colors = base.value(QStringLiteral("colors")).toObject();
    static const QVector<QPair<QString, QString>> names = {{"--bg", "bg"}, {"--bg-2", "bg2"}, {"--bg-3", "bg3"}, {"--bg-4", "bg4"}, {"--fg", "fg"}, {"--fg-2", "fg2"},
                                                           {"--fg-3", "fg3"}, {"--accent", "accent"}, {"--accent-2", "accentHover"}, {"--accent-fg", "accentFg"},
                                                           {"--accent-text", "accentText"}, {"--danger", "danger"}, {"--ok", "ok"}, {"--warn", "warn"},
                                                           {"--control-line", "controlLine"}};
    for (const auto &n : names) {
        const QColor col = cssColor(resolveVar(vars, vars.value(n.first)));
        if (col.isValid() && col.alpha() == 255)
            colors.insert(n.second, col.name(QColor::HexRgb));
    }
    if (!colors.contains(QStringLiteral("focus")) || vars.contains(QStringLiteral("--accent")))
        colors.insert(QStringLiteral("focus"), colors.value(QStringLiteral("accentText")));
    const QJsonObject given = o.value(QStringLiteral("colors")).toObject();
    for (auto it = given.begin(); it != given.end(); ++it)
        colors.insert(it.key(), it.value());
    o.insert(QStringLiteral("colors"), colors);
    if (!o.contains(QStringLiteral("radius"))) {
        bool ok = false;
        const int px = resolveVar(vars, vars.value(QStringLiteral("--radius"))).remove(QStringLiteral("px")).toInt(&ok);
        if (ok)
            o.insert(QStringLiteral("radius"), px);
    }
    const Theme t = fromJson(id, o, true);
    // Readable, as the server's own checks have it: words 4.5 to 1 on the page and on panels.
    if (contrastRatio(t.fg, t.bg) < 4.5 || contrastRatio(t.fg, t.bg2) < 4.5) {
        r.error = QStringLiteral("The text colour is too close to the background colour to read: it needs a contrast of 4.5 to 1 on --bg and --bg-2.");
        return r;
    }
    if (contrastRatio(t.accentFg, t.accent) < 4.5) {
        r.error = QStringLiteral("Text on the accent colour needs a contrast of 4.5 to 1: change --accent-fg or --accent.");
        return r;
    }
    // All is well: unpack into the theme's own folder, replacing an earlier import of the same id.
    const QString dest = QDir(importDir()).filePath(id);
    QDir(dest).removeRecursively();
    QDir().mkpath(dest);
    for (const auto &fi : infos) {
        if (fi.isDir)
            continue;
        const QString target = QDir(dest).filePath(fi.filePath);
        QDir().mkpath(QFileInfo(target).absolutePath());
        QSaveFile out(target);
        if (!out.open(QIODevice::WriteOnly)) {
            r.error = QStringLiteral("Couldn't write %1.").arg(fi.filePath);
            QDir(dest).removeRecursively();
            return r;
        }
        out.write(fi.filePath == QLatin1String("theme.json") ? QJsonDocument(o).toJson(QJsonDocument::Indented) : zip.fileData(fi.filePath));
        out.commit();
    }
    // Fonts the stylesheet names: a font that won't load is noted, and the theme's colours still apply.
    for (const QString &rel : fontUrls) {
        if (QFontDatabase::addApplicationFont(QDir(dest).filePath(rel)) < 0)
            r.notes << QStringLiteral("The font %1 couldn't be loaded, so the usual one is used.").arg(rel);
    }
    r.ok = true;
    r.id = id;
    r.name = t.name;
    return r;
}

// ------------------------------------------------------------------ finishes and scene

void Themes::paintScene(QPainter *p, const Theme &t, const QRectF &r)
{
    if (t.scene.isEmpty() || g_flat)
        return;
    p->save();
    p->setRenderHint(QPainter::Antialiasing);
    p->setClipRect(r);
    for (const auto &lv : t.scene) {
        const QJsonObject l = lv.toObject();
        const QString type = l.value(QStringLiteral("type")).toString();
        const auto gradientStops = [&](QGradient &g) {
            for (const auto &sv : l.value(QStringLiteral("stops")).toArray()) {
                const QJsonObject s = sv.toObject();
                g.setColorAt(qBound(0.0, s.value(QStringLiteral("at")).toDouble() / 100.0, 1.0), Themes::cssColor(s.value(QStringLiteral("color")).toString(), Qt::transparent));
            }
        };
        if (type == QLatin1String("gradient")) {
            const double ang = l.value(QStringLiteral("angle")).toDouble(180) * M_PI / 180.0;
            // CSS angles: 180 is top to bottom.
            const QPointF c = r.center();
            const double len = std::abs(r.width() * std::sin(ang)) + std::abs(r.height() * std::cos(ang));
            const QPointF d(std::sin(ang) * len / 2, -std::cos(ang) * len / 2);
            QLinearGradient g(c - d, c + d);
            gradientStops(g);
            p->fillRect(r, g);
        } else if (type == QLatin1String("radial")) {
            const QPointF c(r.left() + r.width() * l.value(QStringLiteral("x")).toDouble() / 100, r.top() + r.height() * l.value(QStringLiteral("y")).toDouble() / 100);
            const double rx = r.width() * l.value(QStringLiteral("w")).toDouble(50) / 100, ry = r.height() * l.value(QStringLiteral("h")).toDouble(50) / 100;
            QColor col = Themes::cssColor(l.value(QStringLiteral("color")).toString(), QColor(255, 255, 255, 80));
            QRadialGradient g(QPointF(0, 0), 1.0);
            g.setColorAt(0, col);
            col.setAlpha(0);
            g.setColorAt(qBound(0.05, l.value(QStringLiteral("fade")).toDouble(100) / 100, 1.0), col);
            p->save();
            p->translate(c);
            p->scale(rx, ry);
            p->fillRect(QRectF(-1, -1, 2, 2), g);
            p->restore();
        } else if (type == QLatin1String("shape")) {
            const QString kind = l.value(QStringLiteral("kind")).toString();
            const double size = r.width() * l.value(QStringLiteral("size")).toDouble(10) / 100;
            const QPointF c(r.left() + r.width() * l.value(QStringLiteral("x")).toDouble() / 100, r.top() + r.height() * l.value(QStringLiteral("y")).toDouble() / 100);
            if (kind == QLatin1String("circle")) {
                p->setPen(Qt::NoPen);
                p->setBrush(Themes::cssColor(l.value(QStringLiteral("color")).toString(), QColor(255, 255, 255, 60)));
                p->drawEllipse(c, size / 2, size / 2);
            } else if (kind == QLatin1String("hill")) {
                QPainterPath path;
                const double w = r.width() * l.value(QStringLiteral("size")).toDouble(70) / 100 * 1.6, h = r.height() * 0.22;
                path.moveTo(c.x() - w / 2, r.bottom());
                path.quadTo(c.x(), r.bottom() - h * 2, c.x() + w / 2, r.bottom());
                path.closeSubpath();
                QLinearGradient g(c.x(), r.bottom() - h, c.x(), r.bottom());
                gradientStops(g);
                p->setPen(Qt::NoPen);
                p->setBrush(g);
                p->drawPath(path);
            }
        }
    }
    p->restore();
}

QString Themes::finishSheet(const Theme &t)
{
    if (g_flat || !t.finish.any())
        return {};
    const auto rgba = [](const QColor &c) { return QStringLiteral("rgba(%1,%2,%3,%4)").arg(c.red()).arg(c.green()).arg(c.blue()).arg(c.alphaF(), 0, 'f', 2); };
    QString s;
    const Finish &f = t.finish;
    if (f.glass != QLatin1String("none")) {
        const QString fill = QStringLiteral("qlineargradient(x1:0,y1:0,x2:0,y2:1,stop:0 %1,stop:1 %2)").arg(rgba(f.glassHi), rgba(f.glassLo));
        s += QStringLiteral("QFrame[sdui=\"row\"], QFrame[sdui=\"card\"], QFrame[sdui=\"notice\"] { background: %1; border: 1px solid %2; }\n").arg(fill, rgba(f.glassEdge));
        s += QStringLiteral("QWidget#sduiDockHost, QMenuBar, QWidget#controlBar { background: %1; border-top: 1px solid %2; }\n").arg(fill, rgba(f.glassEdge));
        s += QStringLiteral("QMenu { background: %1; border: 1px solid %2; }\n").arg(fill, rgba(f.glassEdge));
    }
    if (f.gloss != QLatin1String("none") && f.glossStops.size() >= 2) {
        // The theme's gloss: a stop at the top, the body, the horizon where the highlight ends, and the foot.
        const auto &g = f.glossStops;
        const QString body = QStringLiteral("qlineargradient(x1:0,y1:0,x2:0,y2:1,stop:0 %1,stop:0.48 %2,stop:0.5 %3,stop:1 %4)")
                                 .arg(g.first().name(), g.value(1).name(), g.value(qMin(2, g.size() - 1)).name(), g.last().name());
        s += QStringLiteral("QPushButton[variant=\"primary\"], QPushButton[role=\"primary\"] { background: %1; border: 1px solid %2; color: #ffffff; }\n").arg(body, g.last().name());
    }
    if (f.bevel != QLatin1String("none")) {
        const QString w = f.bevel == QLatin1String("full") ? QStringLiteral("2px") : QStringLiteral("1px");
        const QColor light = t.bg2.lighter(f.bevel == QLatin1String("full") ? 150 : 125), dark = t.bg4.darker(f.bevel == QLatin1String("full") ? 160 : 125);
        s += QStringLiteral("QPushButton, QMenuBar, QWidget#sduiDockHost { border-style: solid; border-width: %1; border-top-color: %2; border-left-color: %2; border-bottom-color: %3; border-right-color: %3; }\n")
                 .arg(w, light.name(), dark.name());
    }
    return s;
}

} // namespace wmp
