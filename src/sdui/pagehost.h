// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// Gets a page's document from the engine (engine/internal/pages) and draws it in an SduiView,
// and shows the same pages in a dialog. What the window knows comes from a state function.
#pragma once

#include <QDialog>
#include <QJsonObject>
#include <QObject>
#include <QPointer>
#include <QTimer>
#include <functional>

namespace wmp {

class EngineClient;
class SduiView;

class PageHost : public QObject {
    Q_OBJECT
public:
    using StateFn = std::function<QJsonObject()>;
    PageHost(EngineClient *engine, StateFn state, QObject *parent = nullptr);

    // Draws a page in a view. The page is remembered on the view for reload().
    void show(SduiView *view, const QString &page);
    // Asks for the view's page again, with the window's state as it is now.
    void reload(SduiView *view);
    static QString pageOf(const SduiView *view);

    // A document for something that couldn't be made: one Notice.
    static QJsonObject errorDocument(const QString &message);

signals:
    // A node of a view was activated.
    void activated(SduiView *view, const QJsonObject &action);

private:
    EngineClient *engine_;
    StateFn state_;
};

// A page in a window of its own: Settings, Look, a disc's guide, WittyRemote.
class PageDialog : public QDialog {
    Q_OBJECT
public:
    PageDialog(PageHost *host, const QString &page, QWidget *parent);
    SduiView *view() const { return view_; }
    // Reloads when the window is resized: the page's breakpoint depends on its width.
protected:
    void resizeEvent(QResizeEvent *e) override;

private:
    PageHost *host_;
    SduiView *view_;
    QTimer resized_;
};

} // namespace wmp
