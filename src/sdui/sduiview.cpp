// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
#include "sduiview.h"

#include "../ui/theme.h"

#include <QCheckBox>
#include <QEvent>
#include <QFrame>
#include <QHBoxLayout>
#include <QJsonValue>
#include <QKeyEvent>
#include <QLabel>
#include <QLayout>
#include <QMouseEvent>
#include <QPainter>
#include <QStyle>
#include <QStyleOption>
#include <QProgressBar>
#include <QPushButton>
#include <QRegularExpression>
#include <QScrollBar>
#include <QVBoxLayout>
#include <cmath>

namespace wmp {

namespace {

constexpr int kSchema = 1;
constexpr int kCatalog = 7;
constexpr int kMaxNodes = 4000; // a page is a screen, not a library: refuse a runaway document
constexpr int kMaxDepth = 24;

// Left to right, wrapping: a row of buttons or a grid of cards.
class FlowLayout : public QLayout {
public:
    explicit FlowLayout(QWidget *parent, int hGap, int vGap) : QLayout(parent), h_(hGap), v_(vGap) { setContentsMargins(0, 0, 0, 0); }
    ~FlowLayout() override
    {
        qDeleteAll(items_);
    }
    void addItem(QLayoutItem *item) override { items_.append(item); }
    int count() const override { return items_.size(); }
    QLayoutItem *itemAt(int i) const override { return items_.value(i); }
    QLayoutItem *takeAt(int i) override { return i >= 0 && i < items_.size() ? items_.takeAt(i) : nullptr; }
    Qt::Orientations expandingDirections() const override { return {}; }
    bool hasHeightForWidth() const override { return true; }
    int heightForWidth(int w) const override { return place(QRect(0, 0, w, 0), true); }
    void setGeometry(const QRect &r) override
    {
        QLayout::setGeometry(r);
        place(r, false);
    }
    QSize sizeHint() const override { return minimumSize(); }
    QSize minimumSize() const override
    {
        QSize s;
        for (QLayoutItem *i : items_)
            s = s.expandedTo(i->minimumSize());
        const QMargins m = contentsMargins();
        return s + QSize(m.left() + m.right(), m.top() + m.bottom());
    }

private:
    int place(const QRect &r, bool dry) const
    {
        const QMargins m = contentsMargins();
        const QRect area = r.adjusted(m.left(), m.top(), -m.right(), -m.bottom());
        int x = area.x(), y = area.y(), rowH = 0;
        for (QLayoutItem *i : items_) {
            if (i->isEmpty())
                continue;
            const QSize hint = i->sizeHint();
            if (x + hint.width() > area.right() + 1 && rowH > 0) {
                x = area.x();
                y += rowH + v_;
                rowH = 0;
            }
            if (!dry)
                i->setGeometry(QRect(QPoint(x, y), hint));
            x += hint.width() + h_;
            rowH = qMax(rowH, hint.height());
        }
        return y + rowH - r.y() + m.bottom();
    }
    QList<QLayoutItem *> items_;
    int h_, v_;
};

// Anything a person can press: a row, a card, a track. Enter and Space press it.
class ActionFrame : public QFrame {
    Q_OBJECT
public:
    explicit ActionFrame(QWidget *parent) : QFrame(parent)
    {
        setFocusPolicy(Qt::StrongFocus);
        setAttribute(Qt::WA_StyledBackground);
        setCursor(Qt::PointingHandCursor);
    }
signals:
    void pressed();

protected:
    void mouseReleaseEvent(QMouseEvent *e) override
    {
        if (e->button() == Qt::LeftButton && rect().contains(e->position().toPoint()))
            emit pressed();
        QFrame::mouseReleaseEvent(e);
    }
    void keyPressEvent(QKeyEvent *e) override
    {
        if (e->key() == Qt::Key_Return || e->key() == Qt::Key_Enter || e->key() == Qt::Key_Space)
            emit pressed();
        else
            QFrame::keyPressEvent(e);
    }
};

QString str(const QJsonObject &o, const char *k) { return o.value(QLatin1String(k)).toString(); }
QJsonObject obj(const QJsonObject &o, const char *k) { return o.value(QLatin1String(k)).toObject(); }

// The words a person reads, from the small Markdown subset: no marks, links as their text.
QString plain(QString s)
{
    s.replace(QRegularExpression(QStringLiteral(R"(\[([^\]]*)\]\([^)]*\))")), QStringLiteral("\\1"));
    s.remove(QRegularExpression(QStringLiteral(R"((^|\n)#{1,6}\s*)")));
    s.remove(QLatin1Char('*'));
    s.replace(QLatin1Char('\n'), QLatin1Char(' '));
    return s.trimmed();
}

QString joined(const QStringList &parts, const QString &sep = QStringLiteral(", "))
{
    QStringList keep;
    for (const QString &p : parts)
        if (!p.isEmpty())
            keep << p;
    return keep.join(sep);
}

QString actionText(const QJsonObject &a)
{
    const QString t = str(a, "type");
    if (t == QLatin1String("navigate"))
        return QStringLiteral("navigate ") + str(a, "to");
    if (t == QLatin1String("native"))
        return QStringLiteral("native ") + str(a, "to");
    if (t == QLatin1String("external"))
        return QStringLiteral("external ") + str(a, "url");
    if (t == QLatin1String("play"))
        return QStringLiteral("play ") + str(a, "item");
    if (t == QLatin1String("command"))
        return joined({QStringLiteral("command"), str(a, "op"), str(a, "item")}, QStringLiteral(" "));
    return t;
}

QString duration(int sec)
{
    return QStringLiteral("%1:%2").arg(sec / 60).arg(sec % 60, 2, 10, QLatin1Char('0'));
}

} // namespace

struct SduiView::Ctx {
    QJsonObject text, spacing;
    QString breakpoint;
    int nodes = 0;
    QBoxLayout *dockLayout = nullptr;
    QString focusId;
    SduiView *view = nullptr;

    int size(const QString &role) const { return text.value(role).toInt(role == QLatin1String("caption") ? 12 : 14); }
    int gap(const QString &name) const { return spacing.value(name).toInt(8); }
};

const QStringList &SduiView::knownTypes()
{
    static const QStringList t = {QStringLiteral("Page"), QStringLiteral("Stack"), QStringLiteral("Split"), QStringLiteral("Text"), QStringLiteral("Notice"),
                                  QStringLiteral("Button"), QStringLiteral("List"), QStringLiteral("ListItem"), QStringLiteral("Card"), QStringLiteral("Grid"),
                                  QStringLiteral("Track"), QStringLiteral("TrackList"), QStringLiteral("Dock"), QStringLiteral("KeyLegend"),
                                  QStringLiteral("Divider"), QStringLiteral("Spacer")};
    return t;
}

SduiView::SduiView(QWidget *parent) : QWidget(parent)
{
    setObjectName(QStringLiteral("sduiView"));
    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(0);
    scroll_ = new QScrollArea(this);
    scroll_->setWidgetResizable(true);
    scroll_->setFrameShape(QFrame::NoFrame);
    scroll_->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    scroll_->viewport()->setAutoFillBackground(false);
    outer->addWidget(scroll_, 1);
    dockHost_ = new QWidget(this);
    dockHost_->setObjectName(QStringLiteral("sduiDockHost"));
    auto *dl = new QVBoxLayout(dockHost_);
    dl->setContentsMargins(0, 0, 0, 0);
    outer->addWidget(dockHost_);
    dockHost_->hide();
    applyStyle();
}

// The page's own background (the stylesheet's), then the theme's scene over it when it has one.
void SduiView::paintEvent(QPaintEvent *)
{
    QPainter p(this);
    QStyleOption opt;
    opt.initFrom(this);
    style()->drawPrimitive(QStyle::PE_Widget, &opt, &p, this);
    if (!forced_)
        Themes::paintScene(&p, Themes::current(), rect());
}

void SduiView::changeEvent(QEvent *e)
{
    QWidget::changeEvent(e);
    if (e->type() == QEvent::ApplicationPaletteChange || e->type() == QEvent::PaletteChange)
        applyStyle();
}

void SduiView::setForcedTheme(const Theme *theme)
{
    forced_ = theme ? std::make_unique<Theme>(*theme) : nullptr;
    applyStyle();
    update();
}

void SduiView::applyStyle()
{
    const Theme &t = forced_ ? *forced_ : Themes::current();
    const auto h = [](const QColor &c) { return c.name(QColor::HexRgb); };
    const QString sheet = QStringLiteral(R"(
QWidget#sduiView, QWidget#sduiContent, QScrollArea { background: transparent; }
QWidget#sduiDockHost { background: %1; border-top: 1px solid %2; }
QFrame[sdui="row"] { background: %3; border: 2px solid transparent; border-radius: %9px; }
QFrame[sdui="row"]:hover { background: %2; }
QFrame[sdui="row"]:focus { border-color: %4; }
QFrame[sdui="row"][plain="true"] { background: transparent; }
QFrame[sdui="card"] { background: %3; border: 2px solid transparent; border-radius: %9px; }
QFrame[sdui="card"]:hover { background: %2; }
QFrame[sdui="card"]:focus { border-color: %4; }
QFrame[sdui="card"][current="true"] { border-color: %5; }
QFrame[sdui="track"] { background: transparent; border: 2px solid transparent; border-radius: 6px; }
QFrame[sdui="track"]:hover { background: %3; }
QFrame[sdui="track"]:focus { border-color: %4; }
QFrame[sdui="track"][playing="true"] { background: %3; }
QFrame[sdui="notice"] { background: %1; border: 1px solid %2; border-left: 4px solid %5; border-radius: %9px; }
QFrame[sdui="notice"][tone="success"] { border-left-color: %6; }
QFrame[sdui="notice"][tone="warning"] { border-left-color: %7; }
QFrame[sdui="notice"][tone="danger"] { border-left-color: %8; }
QFrame[sdui="dock"] { background: transparent; }
QPushButton[variant="ghost"] { background: transparent; border-color: transparent; }
QPushButton[variant="ghost"]:hover { background: %3; }
QPushButton[dimmed="true"] { background: transparent; color: %10; border: 1px dashed %10; }
QPushButton[tone="danger"] { background: %8; color: %11; border-color: %8; }
QPushButton[variant="primary"] { background: %5; color: %12; border-color: %5; font-weight: 600; }
QPushButton[variant="primary"]:disabled { background: %3; color: %10; border-color: %2; }
QLabel { color: %14; background: transparent; }
QCheckBox { color: %14; }
QLabel[sduiRole="caption"] { color: %13; }
QLabel[sduiRole="muted"] { color: %13; }
)")
                      .arg(h(t.bg2), h(t.bg4), h(t.bg3), h(t.focus), h(t.accent), h(t.ok), h(t.warn), h(t.danger))
                      .arg(t.radius)
                      .arg(h(t.fg3), h(t.bg), h(t.accentFg), h(t.fg2), h(t.fg)) + Themes::finishSheet(t);
    if (sheet != styleSheet()) // a style sheet change must not come back here as a palette change
        setStyleSheet(sheet);
}

QString SduiView::pageId() const { return obj(doc_, "page").value(QStringLiteral("id")).toString(); }

void SduiView::clear()
{
    built_.clear();
    focusables_.clear();
    if (content_) {
        scroll_->takeWidget();
        content_->deleteLater();
        content_ = nullptr;
    }
    QLayout *dl = dockHost_->layout();
    while (QLayoutItem *it = dl->takeAt(0)) {
        delete it->widget();
        delete it;
    }
    dockHost_->hide();
}

bool SduiView::setDocument(const QJsonObject &doc)
{
    if (doc.value(QStringLiteral("version")).toInt() != kSchema || doc.value(QStringLiteral("catalog")).toInt() != kCatalog)
        return false;
    const QJsonObject root = obj(doc, "root");
    if (str(root, "type") != QLatin1String("Page"))
        return false;
    // What had focus, and where the page was scrolled, are kept when the same page is drawn again.
    QString keepFocus;
    if (QWidget *f = window() ? window()->focusWidget() : nullptr)
        if (isAncestorOf(f))
            keepFocus = f->property("sduiId").toString();
    const int keepScroll = (doc.value(QStringLiteral("page")).toObject().value(QStringLiteral("id")) == doc_.value(QStringLiteral("page")).toObject().value(QStringLiteral("id"))) ? verticalScroll() : 0;

    setUpdatesEnabled(false);
    clear();
    doc_ = doc;
    layout_ = obj(doc, "layout");
    Ctx ctx;
    ctx.view = this;
    ctx.text = obj(layout_, "text");
    ctx.spacing = obj(layout_, "spacing");
    ctx.breakpoint = str(layout_, "breakpoint");
    QVBoxLayout *dl = qobject_cast<QVBoxLayout *>(dockHost_->layout());
    ctx.dockLayout = dl;
    content_ = build(root, 0, nullptr, ctx);
    content_->setObjectName(QStringLiteral("sduiContent"));
    scroll_->setWidget(content_);
    dockHost_->setVisible(dl->count() > 0);
    setUpdatesEnabled(true);
    // Tab order is reading order: the widgets were made in it.
    QWidget *prev = nullptr;
    for (QWidget *w : std::as_const(focusables_)) {
        if (prev)
            QWidget::setTabOrder(prev, w);
        prev = w;
    }
    setVerticalScroll(keepScroll);
    if (!keepFocus.isEmpty())
        focusNode(keepFocus);
    emit documentChanged(obj(doc_, "page").value(QStringLiteral("title")).toString());
    return true;
}

int SduiView::verticalScroll() const { return scroll_->verticalScrollBar()->value(); }
void SduiView::setVerticalScroll(int v) { scroll_->verticalScrollBar()->setValue(v); }

bool SduiView::focusNode(const QString &id)
{
    for (QWidget *w : focusables_)
        if (w->property("sduiId").toString() == id && w->isEnabled()) {
            w->setFocus();
            return true;
        }
    return false;
}

QStringList SduiView::focusOrder() const
{
    QStringList out;
    for (QWidget *w : focusables_)
        out << w->property("sduiId").toString();
    return out;
}

QJsonObject SduiView::describe() const
{
    QJsonArray visible;
    for (const SduiBuilt &b : built_) {
        QJsonObject e{{"id", b.id}, {"type", b.type}, {"depth", b.depth}, {"role", b.role}, {"label", b.widget ? b.widget->accessibleName() : b.label}};
        if (!b.action.isEmpty())
            e.insert(QStringLiteral("action"), b.action);
        visible.append(e);
    }
    return {{"kit", 1}, {"visible", visible}};
}

// ----------------------------------------------------------------------- building

QWidget *SduiView::build(const QJsonObject &node, int depth, QWidget *parent, Ctx &ctx)
{
    const QString type = str(node, "type"), id = str(node, "id");
    const QJsonObject p = obj(node, "props");
    const QJsonObject action = obj(node, "action");
    const bool hasAction = !action.isEmpty();
    QJsonArray kids = node.value(QStringLiteral("children")).toArray();

    auto *self = this;
    if (++ctx.nodes > kMaxNodes || depth > kMaxDepth) {
        auto *empty = new QWidget(parent);
        return empty;
    }

    const auto mark = [&](QWidget *w, const QString &role, const QString &label) {
        w->setProperty("sduiId", id);
        w->setProperty("sduiType", type);
        w->setAccessibleName(label);
        SduiBuilt b;
        b.widget = w;
        b.id = id;
        b.type = knownTypes().contains(type) ? type : QStringLiteral("Stack");
        b.role = role;
        b.label = label;
        b.depth = depth;
        if (hasAction)
            b.action = actionText(action);
        built_.append(b);
    };
    const auto wire = [&](ActionFrame *f) {
        focusables_.append(f);
        connect(f, &ActionFrame::pressed, self, [self, action] { emit self->activated(action); });
    };
    const auto tokensStyle = [&](QWidget *w, const QString &selector) {
        const QJsonObject tok = obj(node, "tokens");
        if (tok.isEmpty())
            return;
        const auto c = [&](const char *k, const QString &fallback) {
            const QString v = tok.value(QLatin1String(k)).toString();
            return v.size() >= 7 ? v.left(7) : fallback;
        };
        w->setStyleSheet(QStringLiteral("%1 { background: %2; border: 2px solid %3; }%1:focus { border-color: %4; }%1 QLabel { color: %5; background: transparent; }")
                             .arg(selector, c("bg2", QStringLiteral("#222222")), c("bg4", QStringLiteral("#444444")), c("focus", c("accent", QStringLiteral("#ffffff"))), c("fg", QStringLiteral("#ffffff"))));
    };

    // A vertical box for containers that simply stack their children.
    const auto column = [&](QWidget *w, int spacing) {
        auto *l = new QVBoxLayout(w);
        l->setContentsMargins(0, 0, 0, 0);
        l->setSpacing(spacing);
        return l;
    };
    const auto addKids = [&](QBoxLayout *l, QWidget *host) {
        for (const auto &k : kids) {
            const QJsonObject kid = k.toObject();
            QWidget *w = build(kid, depth + 1, host, ctx);
            if (str(kid, "type") == QLatin1String("Dock")) {
                ctx.dockLayout->addWidget(w);
                continue;
            }
            l->addWidget(w);
        }
    };
    const auto labelOf = [&](const QString &text, const QString &role, QWidget *host) {
        auto *l = new QLabel(text, host);
        l->setWordWrap(true);
        l->setProperty("role", role == QLatin1String("heading") ? QStringLiteral("heading") : QString());
        QFont f = l->font();
        f.setPixelSize(ctx.size(role));
        if (role == QLatin1String("pageTitle") || role == QLatin1String("display") || role == QLatin1String("heading") || role == QLatin1String("title"))
            f.setWeight(QFont::DemiBold);
        l->setFont(f);
        l->setProperty("sduiRole", role == QLatin1String("caption") ? QStringLiteral("caption") : QString());
        return l;
    };

    if (type == QLatin1String("Page")) {
        auto *w = new QWidget(parent);
        auto *outer = new QHBoxLayout(w);
        outer->setContentsMargins(0, 0, 0, 0);
        auto *inner = new QWidget(w);
        inner->setMaximumWidth(1100);
        const int side = ctx.breakpoint == QLatin1String("compact") ? 16 : 28;
        auto *col = column(inner, ctx.gap(QStringLiteral("l")));
        col->setContentsMargins(side, 24, side, 24);
        mark(w, QStringLiteral("main"), str(p, "title"));
        addKids(col, inner);
        col->addStretch(1);
        outer->addWidget(inner, 1);
        outer->addStretch(0);
        return w;
    }
    if (type == QLatin1String("Text")) {
        const QString role = p.value(QStringLiteral("role")).toString(QStringLiteral("body"));
        const QString raw = str(p, "text");
        auto *l = labelOf(raw, role, parent);
        if (str(p, "format") == QLatin1String("markdown")) {
            l->setTextFormat(Qt::MarkdownText);
            l->setOpenExternalLinks(false);
        } else {
            l->setTextFormat(Qt::PlainText);
        }
        const bool head = role == QLatin1String("pageTitle") || role == QLatin1String("heading") || role == QLatin1String("title");
        mark(l, head ? QStringLiteral("heading %1").arg(role == QLatin1String("pageTitle") ? 1 : role == QLatin1String("heading") ? 2 : 3) : QStringLiteral("text"), plain(raw));
        return l;
    }
    if (type == QLatin1String("Divider")) {
        auto *f = new QFrame(parent);
        f->setFrameShape(QFrame::HLine);
        mark(f, QStringLiteral("separator"), QString());
        return f;
    }
    if (type == QLatin1String("Spacer")) {
        auto *w = new QWidget(parent);
        w->setFixedHeight(ctx.gap(p.value(QStringLiteral("size")).toString(QStringLiteral("m"))));
        mark(w, QStringLiteral("none"), QString());
        return w;
    }
    if (type == QLatin1String("Button")) {
        auto *b = new QPushButton(str(p, "label"), parent);
        const QString a11y = str(p, "a11y");
        b->setProperty("variant", str(p, "variant"));
        if (p.value(QStringLiteral("dimmed")).toBool())
            b->setProperty("dimmed", true);
        if (str(p, "tone") == QLatin1String("danger"))
            b->setProperty("tone", QStringLiteral("danger"));
        if (!hasAction) {
            b->setEnabled(false);
        } else {
            focusables_.append(b);
            connect(b, &QPushButton::clicked, self, [self, action] { emit self->activated(action); });
        }
        if (p.value(QStringLiteral("dimmed")).toBool())
            b->setMinimumSize(48, 40);
        mark(b, QStringLiteral("button"), a11y.isEmpty() ? str(p, "label") : a11y);
        b->setAccessibleName(a11y.isEmpty() ? str(p, "label") : a11y);
        b->setProperty("sduiId", id);
        return b;
    }
    if (type == QLatin1String("Stack") || type == QLatin1String("Grid")) {
        auto *w = new QWidget(parent);
        const bool isRow = type == QLatin1String("Grid") || str(p, "direction") == QLatin1String("row");
        const QString gapName = p.value(QStringLiteral("gap")).toString(type == QLatin1String("Grid") ? QStringLiteral("s") : QStringLiteral("m"));
        mark(w, type == QLatin1String("Grid") ? QStringLiteral("grid") : QStringLiteral("none"), type == QLatin1String("Grid") ? str(p, "title") : QString());
        if (isRow) {
            auto *fl = new FlowLayout(w, ctx.gap(gapName), ctx.gap(gapName));
            for (const auto &k : kids)
                fl->addWidget(build(k.toObject(), depth + 1, w, ctx));
            if (type == QLatin1String("Grid") && !str(p, "title").isEmpty()) {
                auto *outer = new QWidget(parent);
                auto *col = column(outer, ctx.gap(QStringLiteral("s")));
                col->addWidget(labelOf(str(p, "title"), QStringLiteral("heading"), outer));
                col->addWidget(w);
                w->setParent(outer);
                return outer;
            }
        } else {
            auto *col = column(w, ctx.gap(gapName));
            addKids(col, w);
        }
        return w;
    }
    if (type == QLatin1String("Split")) {
        auto *w = new QWidget(parent);
        mark(w, QStringLiteral("none"), QString());
        const bool narrow = ctx.breakpoint == QLatin1String("compact") || ctx.breakpoint == QLatin1String("medium");
        QBoxLayout *l = narrow ? static_cast<QBoxLayout *>(new QVBoxLayout(w)) : static_cast<QBoxLayout *>(new QHBoxLayout(w));
        l->setContentsMargins(0, 0, 0, 0);
        l->setSpacing(ctx.gap(p.value(QStringLiteral("gap")).toString(QStringLiteral("m"))));
        const QStringList ratio = str(p, "ratio").split(QLatin1Char(':'));
        int i = 0;
        for (const auto &k : kids) {
            QWidget *c = build(k.toObject(), depth + 1, w, ctx);
            l->addWidget(c, !narrow && ratio.size() == 2 ? qMax(1, ratio[i % 2].toInt()) : 0);
            i++;
        }
        return w;
    }
    if (type == QLatin1String("List") || type == QLatin1String("TrackList")) {
        auto *w = new QWidget(parent);
        auto *col = column(w, 4);
        mark(w, QStringLiteral("list"), str(p, "title"));
        if (!str(p, "title").isEmpty())
            col->addWidget(labelOf(str(p, "title"), QStringLiteral("heading"), w));
        addKids(col, w);
        return w;
    }
    if (type == QLatin1String("Notice")) {
        auto *f = new QFrame(parent);
        f->setAttribute(Qt::WA_StyledBackground);
        f->setProperty("sdui", QStringLiteral("notice"));
        f->setProperty("tone", p.value(QStringLiteral("tone")).toString(QStringLiteral("info")));
        auto *l = new QHBoxLayout(f);
        l->setContentsMargins(14, 10, 14, 10);
        auto *col = new QVBoxLayout;
        col->setSpacing(2);
        if (!str(p, "title").isEmpty()) {
            auto *t = labelOf(str(p, "title"), QStringLiteral("title"), f);
            col->addWidget(t);
        }
        col->addWidget(labelOf(str(p, "text"), QStringLiteral("body"), f));
        l->addLayout(col, 1);
        const QString tone = str(p, "tone");
        mark(f, tone == QLatin1String("warning") || tone == QLatin1String("danger") ? QStringLiteral("alert") : QStringLiteral("status"),
             str(p, "title").isEmpty() ? str(p, "text") : str(p, "title") + QStringLiteral(". ") + str(p, "text"));
        return f;
    }
    if (type == QLatin1String("ListItem")) {
        auto *f = new ActionFrame(parent);
        f->setProperty("sdui", QStringLiteral("row"));
        f->setProperty("plain", !hasAction);
        if (!hasAction)
            f->setFocusPolicy(Qt::NoFocus);
        auto *l = new QHBoxLayout(f);
        l->setContentsMargins(14, 8, 14, 8);
        l->setSpacing(12);
        auto *col = new QVBoxLayout;
        col->setSpacing(1);
        col->addWidget(labelOf(str(p, "title"), QStringLiteral("body"), f));
        if (!str(p, "subtitle").isEmpty())
            col->addWidget(labelOf(str(p, "subtitle"), QStringLiteral("caption"), f));
        l->addLayout(col, 1);
        if (!str(p, "trailing").isEmpty()) {
            auto *t = labelOf(str(p, "trailing"), QStringLiteral("caption"), f);
            t->setWordWrap(false);
            l->addWidget(t);
        }
        if (hasAction && str(action, "type") == QLatin1String("navigate")) {
            auto *chev = new QLabel(QStringLiteral("›"), f);
            chev->setProperty("sduiRole", QStringLiteral("caption"));
            l->addWidget(chev);
        }
        if (hasAction)
            wire(f);
        mark(f, hasAction ? QStringLiteral("button") : QStringLiteral("text"), joined({str(p, "title"), str(p, "subtitle"), str(p, "trailing")}));
        f->setProperty("sduiId", id);
        return f;
    }
    if (type == QLatin1String("Card")) {
        auto *f = new ActionFrame(parent);
        f->setProperty("sdui", QStringLiteral("card"));
        f->setFixedWidth(180);
        const QJsonArray badges = p.value(QStringLiteral("badges")).toArray();
        f->setProperty("current", !badges.isEmpty());
        auto *col = new QVBoxLayout(f);
        col->setContentsMargins(12, 12, 12, 12);
        col->setSpacing(4);
        // No picture to ask for here: the card shows a tile with the first letter, in the card's own colours.
        auto *tile = new QLabel(str(p, "title").left(1).toUpper(), f);
        tile->setAlignment(Qt::AlignCenter);
        tile->setFixedHeight(72);
        QFont tf = tile->font();
        tf.setPixelSize(30);
        tf.setWeight(QFont::DemiBold);
        tile->setFont(tf);
        col->addWidget(tile);
        col->addWidget(labelOf(str(p, "title"), QStringLiteral("title"), f));
        if (!str(p, "subtitle").isEmpty())
            col->addWidget(labelOf(str(p, "subtitle"), QStringLiteral("caption"), f));
        tokensStyle(f, QStringLiteral("QFrame[sdui=\"card\"]"));
        if (hasAction)
            wire(f);
        mark(f, QStringLiteral("button"), joined({str(p, "title"), str(p, "subtitle")}));
        f->setProperty("sduiId", id);
        return f;
    }
    if (type == QLatin1String("Track")) {
        auto *f = new ActionFrame(parent);
        f->setProperty("sdui", QStringLiteral("track"));
        f->setProperty("playing", p.value(QStringLiteral("playing")).toBool());
        if (!hasAction)
            f->setFocusPolicy(Qt::NoFocus);
        auto *l = new QHBoxLayout(f);
        l->setContentsMargins(10, 6, 10, 6);
        l->setSpacing(10);
        const QJsonObject toggle = obj(p, "toggle");
        const int number = p.value(QStringLiteral("number")).toInt();
        const QString title = str(p, "title");
        if (!toggle.isEmpty()) {
            auto *cb = new QCheckBox(f);
            cb->setChecked(p.value(QStringLiteral("checked")).toBool());
            cb->setAccessibleName(QStringLiteral("Copy track %1, %2").arg(number).arg(title));
            const QJsonObject tact{{"type", "command"}, {"op", str(toggle, "op")}, {"item", str(toggle, "item")}};
            connect(cb, &QCheckBox::clicked, self, [self, tact] { emit self->activated(tact); });
            cb->setProperty("sduiId", id + QStringLiteral("~tick"));
            focusables_.append(cb);
            l->addWidget(cb);
        }
        auto *num = labelOf(QString::number(number), QStringLiteral("caption"), f);
        num->setFixedWidth(26);
        num->setAlignment(Qt::AlignRight | Qt::AlignVCenter);
        l->addWidget(num);
        auto *col = new QVBoxLayout;
        col->setSpacing(1);
        col->addWidget(labelOf(title, QStringLiteral("body"), f));
        const QString artist = str(p, "artist");
        const QString note = joined({artist, str(p, "note"), str(p, "result")}, QStringLiteral(" · "));
        if (!note.isEmpty())
            col->addWidget(labelOf(note, QStringLiteral("caption"), f));
        l->addLayout(col, 1);
        if (p.contains(QStringLiteral("progress"))) {
            auto *bar = new QProgressBar(f);
            bar->setRange(0, 100);
            bar->setValue(int(std::round(p.value(QStringLiteral("progress")).toDouble() * 100)));
            bar->setTextVisible(false);
            bar->setFixedSize(90, 8);
            bar->setAccessibleName(QStringLiteral("Copy progress"));
            l->addWidget(bar);
        }
        const int dur = p.value(QStringLiteral("durationSec")).toInt();
        if (dur > 0) {
            auto *d = labelOf(duration(dur), QStringLiteral("caption"), f);
            d->setWordWrap(false);
            l->addWidget(d);
        }
        if (hasAction)
            wire(f);
        mark(f, QStringLiteral("button"),
             QStringLiteral("Track %1, %2%3, %4 minutes %5 seconds").arg(number).arg(title).arg(artist.isEmpty() ? QString() : QStringLiteral(", ") + artist).arg(dur / 60).arg(dur % 60));
        f->setProperty("sduiId", id);
        return f;
    }
    if (type == QLatin1String("KeyLegend")) {
        auto *l = labelOf(QStringLiteral("Enter  Select      Esc  Back      Tab  Next"), QStringLiteral("caption"), parent);
        l->setWordWrap(false);
        mark(l, QStringLiteral("none"), QString());
        l->setAccessibleName(QString()); // hidden from assistive technology: each control carries its own hint
        return l;
    }
    if (type == QLatin1String("Dock")) {
        auto *f = new QFrame(parent);
        f->setProperty("sdui", QStringLiteral("dock"));
        auto *l = new QHBoxLayout(f);
        l->setContentsMargins(24, 8, 24, 8);
        l->setSpacing(ctx.gap(QStringLiteral("s")));
        mark(f, QStringLiteral("toolbar"), p.value(QStringLiteral("title")).toString(QStringLiteral("Options")));
        QWidget *legend = nullptr;
        for (const auto &k : kids) {
            QWidget *w = build(k.toObject(), depth + 1, f, ctx);
            if (str(k.toObject(), "type") == QLatin1String("KeyLegend"))
                legend = w;
            else
                l->addWidget(w);
        }
        l->addStretch(1);
        if (legend)
            l->addWidget(legend);
        return f;
    }
    // A component this renderer doesn't know: a plain column of what is inside it.
    auto *w = new QWidget(parent);
    auto *col = column(w, ctx.gap(QStringLiteral("s")));
    mark(w, QStringLiteral("none"), QString());
    addKids(col, w);
    return w;
}

} // namespace wmp

#include "sduiview.moc"
