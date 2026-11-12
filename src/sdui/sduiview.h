// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The SDUI renderer: draws a WittyMediaServer page document (catalogue revision 7) with Qt
// widgets. The documents come from the engine's local page provider (engine/internal/pages)
// or, in time, from a server; the renderer can't tell the two apart. Components it doesn't
// know are drawn as a plain column of their children, as the contract says.
#pragma once

#include <QJsonArray>
#include <QJsonObject>
#include <QScrollArea>
#include <QString>
#include <QVector>

#include "../ui/theme.h"
#include <memory>

class QBoxLayout;
class QFrame;
class QLabel;
class QVBoxLayout;

namespace wmp {

// One node as built: what the conformance test reads back.
struct SduiBuilt {
    QWidget *widget = nullptr;
    QString id, type, role, label, action;
    int depth = 0;
};

class SduiView : public QWidget {
    Q_OBJECT
public:
    explicit SduiView(QWidget *parent = nullptr);

    // Draws a document. Focus stays on the same node id when it is still there.
    // False (and nothing changes) when the document is for another schema or catalogue.
    bool setDocument(const QJsonObject &doc);
    const QJsonObject &document() const { return doc_; }
    QString pageId() const;

    // The component types this renderer draws itself; any other is a plain column.
    static const QStringList &knownTypes();

    // What was built, depth first in display order.
    const QVector<SduiBuilt> &built() const { return built_; }
    // The normalised layout of the conformance kit (a subset of it: id, type, depth, role,
    // label, action), described from the widgets as built.
    QJsonObject describe() const;
    // The ids of everything that takes focus, in focus order.
    QStringList focusOrder() const;

    // Draws in this theme instead of the application's (the CD screen's classic look); null goes back.
    void setForcedTheme(const Theme *theme);
    // Moves focus to a node by id. False when there is none.
    bool focusNode(const QString &id);
    void setVerticalScroll(int value);
    int verticalScroll() const;

signals:
    // A new document is on show: its page's title.
    void documentChanged(const QString &title);
    // A node was activated: the action as the document gave it ({type, to, op, item, params}).
    void activated(const QJsonObject &action);

protected:
    void changeEvent(QEvent *e) override;
    void paintEvent(QPaintEvent *e) override;

private:
    struct Ctx;
    QWidget *build(const QJsonObject &node, int depth, QWidget *parent, Ctx &ctx);
    void clear();
    void applyStyle();

    QJsonObject doc_;
    QScrollArea *scroll_ = nullptr;
    QWidget *content_ = nullptr;
    QWidget *dockHost_ = nullptr;
    QVector<SduiBuilt> built_;
    QVector<QWidget *> focusables_;
    QJsonObject layout_;
    std::unique_ptr<Theme> forced_;
};

} // namespace wmp
